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

def json_is_included_expectedly(expected, given):
    if type(expected) != type(given):
        return False

    if type(expected) == type({}):
        for k, v in expected.items():
            actual = given.get(k, None)
            if type(v) != type(actual):
                return False 
            if type(v) == type([]):
                for n in v:
                    if not n in actual:
                        return False
            elif type(v) == type({}):
                included = json_is_included_expectedly(v, actual)
                if not included:
                    return False
            else:
                if actual != v:
                    return False
    elif type(expected) == type([]):
        for n in expected:
            if not n in given:
                return False
    else:
        if expected != given:
            return False
    return True

def curl_verify(name, req, expected_resp):
    method = req.get('method', 'GET')
    queries = req.get('queries', {})
    url = req['url']
    headers = req.get('headers', {})
    body = req.get('body', {})
    expected_status = expected_resp.get('status_code', 200)
    expected_headers = expected_resp.get('headers', {}) 
    expected_body = expected_resp.get('body', {})
    try:
        warn(name, "requesting: %s" % req)
        resp = requests.request(method=method, url="%s" % url, params=queries, headers=headers, json=body, allow_redirects=False, timeout=2)
        try:
            resp_headers = dict(resp.headers)
            if type(expected_body) == type({}):
                resp_body = resp.json()
            else:
                resp_body = resp.text
        except Exception as e:
            return False, "Response format unexpected: not json-formated(%s)" % e

        if resp.status_code != expected_status:
            return False, "Status code unexpected: expected: %d, actually: %d" % (expected_status, resp.status_code)
        if not json_is_included_expectedly(expected_headers, resp_headers):
            return False, "Header unexpected: expected %s, actually %s " % (expected_headers, resp_headers)
        
        if not json_is_included_expectedly(expected_body, resp_body):
            return False, "Response body unexpected: expected %s, actually %s" % (expected_body, resp_body)

    except Exception as e:
        return False, "failed to %s to %s: %s" % (method, url, e)
    else:
        return True, "Successfully verified %s via %s %s" % (name, method, url)


class test_context():
    def __init__(self, name, ctx_yamls) -> None:
        self.ctxs = ctx_yamls
        self.name = name
    def __enter__(self):
        for ctx in self.ctxs:
            gen_kube_yaml(self.name, ctx)
            apply_kube_yaml(self.name, '%s/deps/%s.yaml' % (homedir, ctx))
    def __exit__(self, exc_type, exc_val, exc_tb):
        for ctx in self.ctxs:
            delete_kube_yaml(self.name, '%s/deps/%s.yaml' % (homedir, ctx))

os.environ.setdefault('KUBE_CONFIG_FILEPATH', '~/.kube/config')
load_config()
load_testcases()

for case in testcases:
    n = case['name']
    note(n, "Testing %s" % n)
    
    with test_context(n, case['context']):
        retries = 50
        for t in range(retries):
            (passed, msg)  = curl_verify(n, case['request'], case['response'])
            if not passed:
                time.sleep(2)
                warn(n, msg)
                warn(n, "Another retry: %d" % (retries-t))
                if t == retries-1:
                    fail(n, "Timeout for testing... quit.")
            else:
                ok(n, msg)
                break
