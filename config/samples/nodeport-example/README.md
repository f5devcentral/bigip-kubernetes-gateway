curl --resolve my.test.com:80:10.250.16.127 http://my.test.com/foxfox
curl --resolve my.test.com:80:10.250.16.127 http://my.test.com/tmp

watch -n 1 "curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/virtual | python -m json.tool | grep name"
watch -n 1 "curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/pool | python -m json.tool | grep name"
watch -n 1 'curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/rule | python -m json.tool | grep \"name\":'