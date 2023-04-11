package helpers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

type CAPEM []byte
type CAPrivateKey []byte
type ServerCertPEM []byte
type ServerPrivateKey []byte

var default_ca = &x509.Certificate{
	SerialNumber: big.NewInt(2019),
	Subject: pkix.Name{
		Organization:  []string{"F5"},
		Country:       []string{"US"},
		Province:      []string{""},
		Locality:      []string{"Seattle"},
		StreetAddress: []string{"WA Corporate HQ 801 5th Ave"},
		PostalCode:    []string{"98104"},
	},
	NotBefore:             time.Now(),
	NotAfter:              time.Now().AddDate(10, 0, 0),
	IsCA:                  true,
	ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	BasicConstraintsValid: true,
}

func sign(crt, ca *x509.Certificate, caPrivKey *rsa.PrivateKey) ([]byte, []byte, error) {

	rawCrtBytes, err := x509.CreateCertificate(rand.Reader, crt, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	crtPem := &bytes.Buffer{}
	if err = pem.Encode(crtPem, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCrtBytes,
	}); err != nil {
		return nil, nil, err
	}
	crtPrivKeyPem := &bytes.Buffer{}
	if err = pem.Encode(crtPrivKeyPem, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	}); err != nil {
		return nil, nil, err
	}

	return crtPem.Bytes(), crtPrivKeyPem.Bytes(), nil
}

func GenerateCA(ca *x509.Certificate) (CAPEM, CAPrivateKey, error) {
	if ca == nil {
		ca = default_ca
	}

	rawPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	return sign(ca, ca, rawPrivKey)
}

var default_cert = &x509.Certificate{
	SerialNumber: big.NewInt(2019),
	Subject: pkix.Name{
		Organization:  []string{"F5, Dev."},
		Country:       []string{"CN"},
		Province:      []string{""},
		Locality:      []string{"Beijing"},
		StreetAddress: []string{"Jiang Guo Road"},
		PostalCode:    []string{"100000"},
	},
	IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	NotBefore:    time.Now(),
	NotAfter:     time.Now().AddDate(10, 0, 0),
	SubjectKeyId: []byte{1, 2, 3, 4, 5},
	ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	KeyUsage:     x509.KeyUsageDigitalSignature,
}

func GenerateServerCert(serverCert *x509.Certificate, caPem CAPEM, caPrivKeyPem CAPrivateKey) (ServerCertPEM, ServerPrivateKey, error) {

	caPemBlock, _ := pem.Decode(caPem)
	if caPemBlock == nil {
		return nil, nil, fmt.Errorf("can not decode CA Certificate")
	}
	ca, err := x509.ParseCertificate(caPemBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	caPrivKeyBlock, _ := pem.Decode(caPrivKeyPem)
	if caPrivKeyBlock == nil {
		return nil, nil, fmt.Errorf("can not decode CA Private Key")
	}
	caPrivKey, err := x509.ParsePKCS1PrivateKey(caPrivKeyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	if serverCert == nil {
		serverCert = default_cert
	}

	return sign(serverCert, ca, caPrivKey)
}

func VerifyServerWithCA(caPem []byte, serverPem []byte) error {
	caBlock, _ := pem.Decode(caPem)
	if caBlock == nil {
		return fmt.Errorf("can not decode CA Certificate")
	}

	serverBlock, _ := pem.Decode(serverPem)
	if serverBlock == nil {
		return fmt.Errorf("can not decode Server Certificate")
	}

	serverCert, err := x509.ParseCertificate(serverBlock.Bytes)
	if err != nil {
		return err
	}

	root := x509.NewCertPool()
	if !root.AppendCertsFromPEM(caPem) {
		return fmt.Errorf("can not add root Certificate")
	}

	if _, err := serverCert.Verify(x509.VerifyOptions{
		Roots: root,
	}); err != nil {
		return err
	}

	return nil
}
