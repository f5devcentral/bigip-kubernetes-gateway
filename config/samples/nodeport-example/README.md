curl --resolve cafe.example.com:80:10.250.16.127 http://cafe.example.com/coffee
curl --resolve cafe.example.com:80:10.250.16.127 http://cafe.example.com/tea

watch -n 1 "curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/virtual | python -m json.tool | grep name"
watch -n 1 "curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/pool | python -m json.tool | grep name"
watch -n 1 'curl -k -u admin:P@ssw0rd123 https://10.250.15.180/mgmt/tm/ltm/rule | python -m json.tool | grep \"name\":'