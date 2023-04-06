# HTTPS Gateway

 We can't talk about Gateway without talking about Terminated HTTPS.
 
 With Terminated HTTPS, we can let the gateway(BIG-IP in this implementation) to do the TLS offload, making the backend services focusing on business processing.

 In this example, let's go through this use case.

 ## Run the Example

 In *secret.yaml* file, we provide a `Secret` with the tls cert and key. You may need to replace them with your owns.

 A recommended way to generate the `Secret` is [cert-manager.io](https://gateway-api.sigs.k8s.io/implementations/?h=cert+manager#cert-manager).

 ### 1. Create the `Secret`

 ```shell
 $ kubectl apply -f sercret.yaml
 ```

 In *secret.yaml*, we also define `Namespace` `ReferenceGrant` to demonstrate a cross namespace reference. This is closer to user scenarios that `Secret`s are usually be stored in a protected namespace. 

 ### 2. Create the `Service`

 ```shell
 $ kubectl apply -f service.yaml
 ```

 The `Service` is as usual as that in [Simple Gateway](../simple-gateway/) case.

 ### 3. Create other Gateway API Resources

 ```shell
 $ kubectl apply -f gatewayapis.yaml
 ```

With *gatewayapis.yaml*, we create `GatewayClass`, `Gateway` and `HTTPRoute` in order.

## Verify the Deployed Gateway

Access the https gateway:

```shell
$ curl -k https://10.250.17.143/path-test -H "Host: gateway.api"
{"queries":{},"headers":{"Host":"gateway.api","User-Agent":"curl/7.86.0","Accept":"*/*"},"version":"1.1","method":"GET","remote-address":"10.42.20.1","uri":"/path-test","server_name":"bigip.test.service"}
```

*You may also use `--cacert` ca.crt to appoint the CA certificate for verifying server certificate since the server certificate may be self-signed.*

### Verify the Server Certificate Details

```shell
$ openssl s_client -connect 10.250.17.143:443 -showcerts
CONNECTED(00000003)
depth=0 C = CN, ST = BJ, L = beijing, O = f5, OU = zong.f5.com, CN = a.zong.f5.com, emailAddress = a.zong@f5.com
verify error:num=18:self signed certificate
verify return:1
depth=0 C = CN, ST = BJ, L = beijing, O = f5, OU = zong.f5.com, CN = a.zong.f5.com, emailAddress = a.zong@f5.com
verify error:num=10:certificate has expired
notAfter=Jan  6 03:06:37 2022 GMT
verify return:1
depth=0 C = CN, ST = BJ, L = beijing, O = f5, OU = zong.f5.com, CN = a.zong.f5.com, emailAddress = a.zong@f5.com
notAfter=Jan  6 03:06:37 2022 GMT
verify return:1
write W BLOCK
---
Certificate chain
 0 s:/C=CN/ST=BJ/L=beijing/O=f5/OU=zong.f5.com/CN=a.zong.f5.com/emailAddress=a.zong@f5.com
   i:/C=CN/ST=BJ/L=beijing/O=f5/OU=zong.f5.com/CN=a.zong.f5.com/emailAddress=a.zong@f5.com
-----BEGIN CERTIFICATE-----
MIIDiDCCAnACCQCwsetXAEnCoDANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC
Q04xCzAJBgNVBAgMAkJKMRAwDgYDVQQHDAdiZWlqaW5nMQswCQYDVQQKDAJmNTEU
MBIGA1UECwwLem9uZy5mNS5jb20xFjAUBgNVBAMMDWEuem9uZy5mNS5jb20xHDAa
BgkqhkiG9w0BCQEWDWEuem9uZ0BmNS5jb20wHhcNMjExMjA3MDMwNjM3WhcNMjIw
MTA2MDMwNjM3WjCBhTELMAkGA1UEBhMCQ04xCzAJBgNVBAgMAkJKMRAwDgYDVQQH
DAdiZWlqaW5nMQswCQYDVQQKDAJmNTEUMBIGA1UECwwLem9uZy5mNS5jb20xFjAU
BgNVBAMMDWEuem9uZy5mNS5jb20xHDAaBgkqhkiG9w0BCQEWDWEuem9uZ0BmNS5j
b20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDgHLExdv8aok2MlQJn
A7PI2+g/VUdAa2/cf7IkUtgv7XhDgk7OBcBw1ggNqAgLTqsY3o48aHUhAlQcLtSs
h7XYxlbrdTLhiQc/DSqaf4yxlJ139RJ6qMlBBilkSCRtGBv7DUxGByxdweHr5Zwf
qIGNw5f3lm5oF6htXL49sgYzUlljuiNhrq7eFZFBStUHNhobYUA8ZeOk/WyMDrup
sb8VCLv/eAjxixNQeonlZC1DY/qpCATd/xYVOEyR6tDb41bXehmqbpFWnOumPEud
zXI+O9q58tXfFTuUtGXP3xgaHaxSs9xk/Iqn5bpizlwIlLMSs/wVr/uCQbLMvAAL
wvKbAgMBAAEwDQYJKoZIhvcNAQEFBQADggEBAMYEWzDtbjYlqOWtuaCUw2S3fJFB
+IWwWVsu0LN6+a1OPIDpAb5/ueKGv4CNDi5bXLTSP2or4CsWpz4mzK0Zp3Gt12uG
NeOI4IUOd+5c+X+eF8fuW42luXBXHsNZPT7HNFCVV2XmhtOE90TsR/qPVH4llDlJ
K+aHk9dd/PCSgG6+S7wwQhUxLdM7prns6RmPbUT2Sr3r57jS3JJF56Ejk2/LLmNK
XHPO+a5hOBuEcwpEtuBB64/uuY1z+5vLddFl/8snHbWAZEQdUD1k9Vo7XXmP/6ac
fYc0k0zdhJcqg45ftWumTWuPBVxx78TdK6k5nN9+TDgB7gSnjCyu4JyAl0w=
-----END CERTIFICATE-----
---
Server certificate
subject=/C=CN/ST=BJ/L=beijing/O=f5/OU=zong.f5.com/CN=a.zong.f5.com/emailAddress=a.zong@f5.com
issuer=/C=CN/ST=BJ/L=beijing/O=f5/OU=zong.f5.com/CN=a.zong.f5.com/emailAddress=a.zong@f5.com
---
No client certificate CA names sent
Server Temp Key: ECDH, P-256, 256 bits
---
SSL handshake has read 1413 bytes and written 413 bytes
---
New, TLSv1/SSLv3, Cipher is ECDHE-RSA-AES128-GCM-SHA256
Server public key is 2048 bit
Secure Renegotiation IS supported
Compression: NONE
Expansion: NONE
No ALPN negotiated
SSL-Session:
    Protocol  : TLSv1.2
    Cipher    : ECDHE-RSA-AES128-GCM-SHA256
    Session-ID: 80DFB369B58206AD28679B31DC8B6EF44E39B1569845E592F02644A52C9B3E85
    Session-ID-ctx:
    Master-Key: 1076BE0419F11D713BE98913481C861EB2DB5012279DDA861095DE4460DF05633ED12A8CFEB6050E17937A938803617D
    Start Time: 1680753364
    Timeout   : 7200 (sec)
    Verify return code: 10 (certificate has expired)
---
```