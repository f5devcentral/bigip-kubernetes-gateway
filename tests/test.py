import base64
import getopt
import subprocess
import json
import os
import sys
import time
import yaml
import jinja2
import requests
import glob
import urllib3
from urllib3.exceptions import InsecureRequestWarning
urllib3.disable_warnings(InsecureRequestWarning)

homedir = os.path.abspath(os.path.dirname(sys.argv[0]))

config_file = 'config.yaml'

config = {}
testcases = []

def load_config():
    global config
    with open(os.path.join(homedir, config_file)) as fr:
        config = yaml.safe_load(fr)

def load_testcases():
    global testcases
    note("test setup", "Loading test cases...")
    with open(os.path.join(homedir, 'templates/testcases.yaml.j2')) as fr:
        tmpl = jinja2.Template(fr.read())
        yaml_content = tmpl.render(**config)
        with open('%s/deps/testcases.yaml' % homedir, 'w') as fw:
            fw.write(yaml_content)
    with open('%s/deps/testcases.yaml' % homedir) as fr:
        testcases = yaml.safe_load(fr)
    ok("test setup", "Loaded %d test cases from %s" % (len(testcases), fr.name))

def gen_kube_yaml(name, case_file):
    warn(name, "Generating ... %s" % case_file)
    with open(os.path.join(homedir, 'templates/%s.yaml.j2' % case_file)) as fr:
        tmpl = jinja2.Template(fr.read())
        yaml_content = tmpl.render(**config)
        with open(os.path.join(homedir, 'deps/%s.yaml' % case_file), 'w') as fw:
            fw.write(yaml_content)
            ok(name, "Generated %s" % fw.name)

def fail(name, msg):
    lines = msg.split("\n")
    for line in lines:
        print("  \033[1;35mFailed\033[0m : %-30s %s" % (name, line))
    raise Exception("Details")

def ok(name, msg=""):
    lines = msg.split("\n")
    for line in lines:
        print("  \033[1;32mOK\033[0m     : %-30s %s" % (name, line))

def note(name, msg=""):
    lines = msg.split("\n")
    for line in lines:
        print("  ->     : %-30s %s" % (name, line))

def warn(name, msg):
    lines = msg.split("\n")
    for line in lines:
        print("  \033[1;30m...\033[0m    : %-30s %s" % (name, line))

def apply_kube_yaml(name, yaml_file):
    cmd = "kubectl --kubeconfig %s apply -f %s" % (os.environ['KUBE_CONFIG_FILEPATH'], yaml_file)
    warn(name, "Deploying ... %s" % cmd)
    cp = subprocess.run(cmd, shell=True, stderr=subprocess.PIPE, stdout=subprocess.PIPE)
    # print(cp)
    if cp.returncode != 0:
        fail(name, "Failed to deploy: %s" % str(cp.stderr, 'utf-8'))
    else:
        ok(name, "%s" % str(cp.stdout, 'utf-8'))


def delete_kube_yaml(name, yaml_file):
    cmd = "kubectl --kubeconfig %s delete -f %s" % (os.environ['KUBE_CONFIG_FILEPATH'], yaml_file)
    warn(name, "Deleting ... %s" % cmd)
    cp = subprocess.run(cmd, shell=True, stderr=subprocess.PIPE, stdout=subprocess.PIPE)
    # print(cp)
    if cp.returncode != 0:
        fail(name, "Failed to delete: %s" % str(cp.stderr, 'utf-8'))
    else:
        ok(name, "%s" % str(cp.stdout, 'utf-8'))

def curl_verify(name, req, expected_resp):
    method = req.get('method', 'GET')
    queries = req.get('queries', {})
    url = req['url']
    headers = req.get('headers', {})
    body = req.get('body', {})
    expected_status = expected_resp.get('status_code', 200)
    expected_headers = expected_resp.get('headers', {}) 
    expected_body_json = expected_resp.get('body', {})
    try:
        resp = requests.api.request(method=method, url="%s" % url, params=queries, headers=headers, json=body)
        if resp.status_code != expected_status:
            fail(name, "Status code unexpected: expected: %d, actually: %d" % (expected_status, resp.status_code))
        for k, v in expected_headers.items():
            if resp.headers[k] != v:
                fail(name, "Header unexpected: expected %s => %s, actually %s => %s" % (k, v, k, resp.headers[k]))
        try:
            resp_json = resp.json()
        except Exception as e:
            fail(name, "Response format unexpected: not json-formated(%s)" % e)
        if type(expected_body_json) != type(resp_json):
            fail(name, "Response format unexpected: expected: %s, actually: %s" % (type(expected_body_json), type(resp_json)))
        if type(expected_body_json) == type([]):
            for n in expected_body_json:
                if not n in resp_json:
                    fail(name, "Response body unexpected, missing %s" % n)
        elif type(expected_body_json) == type({}):
            for k, v in expected_body_json.items():
                if v != resp_json[k]:
                    fail(name, "Response body unexpected: expected %s => %s, actually %s => %s" % (k, v, k, resp_json[k]))
    except Exception as e:
        fail(name, "failed to %s to %s: %s" % (method, url, e))
    else:
        ok(name, "Successfully verified %s via %s %s" % (name, method, url))

os.environ.setdefault('KUBE_CONFIG_FILEPATH', '~/.kube/config')
load_config()
load_testcases()

for case in testcases:
    n = case['name']
    note(n, "Testing %s" % n)
    for ctx in case['context']:
        gen_kube_yaml(n, ctx)
        apply_kube_yaml(n, '%s/deps/%s.yaml' % (homedir, ctx))

    retries = 50
    for t in range(retries):
        try:
            curl_verify(n, case['request'], case['response'])
        except Exception as e:
            time.sleep(2)
            warn(n, "Another retry: %d" % (retries-t))
            if t == retries-1:
                fail(n, "Timeout for testing... quit.")
        else:
            break

    for ctx in case['context']:
        delete_kube_yaml(n, '%s/deps/%s.yaml' % (homedir, ctx))
