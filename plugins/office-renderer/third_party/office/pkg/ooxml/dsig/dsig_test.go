package dsig_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"office/pkg/ooxml/dsig"
)

func TestParseSignature(t *testing.T) {
	certDER := generateTestCert(t)
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	xml := `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="">
      <Transforms>
        <Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/>
      </Transforms>
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
  <KeyInfo>
    <X509Data>
      <X509Certificate>` + certB64 + `</X509Certificate>
    </X509Data>
  </KeyInfo>
</Signature>`

	sig, err := dsig.ParseSignature([]byte(xml))
	if err != nil {
		t.Fatalf("parse signature: %v", err)
	}

	if sig.ID != "sig1" {
		t.Errorf("ID = %q, want sig1", sig.ID)
	}
	if got := sig.SignatureMethodURI(); got != "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" {
		t.Errorf("SignatureMethod = %q", got)
	}
	if got := sig.CanonicalizationMethodURI(); got != "http://www.w3.org/TR/2001/REC-xml-c14n-20010315" {
		t.Errorf("CanonicalizationMethod = %q", got)
	}
	if len(sig.SignedInfo.References) != 1 {
		t.Fatalf("References = %d, want 1", len(sig.SignedInfo.References))
	}
	ref := sig.SignedInfo.References[0]
	if ref.URI != "" {
		t.Errorf("Reference.URI = %q", ref.URI)
	}
	if ref.DigestMethod.Algorithm != "http://www.w3.org/2001/04/xmlenc#sha256" {
		t.Errorf("DigestMethod = %q", ref.DigestMethod.Algorithm)
	}
	if len(ref.Transforms.Transform) != 1 {
		t.Fatalf("Transforms = %d, want 1", len(ref.Transforms.Transform))
	}
	if ref.Transforms.Transform[0].Algorithm != "http://www.w3.org/2000/09/xmldsig#enveloped-signature" {
		t.Errorf("Transform = %q", ref.Transforms.Transform[0].Algorithm)
	}

	certs := sig.Certificates()
	if len(certs) != 1 {
		t.Fatalf("Certificates = %d, want 1", len(certs))
	}
	if certs[0].Subject.CommonName != "test-signer" {
		t.Errorf("CN = %q", certs[0].Subject.CommonName)
	}

	signer := sig.Signer()
	if signer == nil {
		t.Fatal("Signer() = nil")
	}
	if signer.SerialNumber.Cmp(certs[0].SerialNumber) != 0 {
		t.Error("Signer serial mismatch")
	}
}

func TestParseSignature_NoKeyInfo(t *testing.T) {
	xml := `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
</Signature>`

	sig, err := dsig.ParseSignature([]byte(xml))
	if err != nil {
		t.Fatalf("parse signature: %v", err)
	}
	if sig.Signer() != nil {
		t.Error("expected nil signer")
	}
	if sig.Certificates() != nil {
		t.Error("expected nil certificates")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	xml := `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
</Signature>`

	sig, err := dsig.ParseSignature([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, err := sig.MarshalXML()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	sig2, err := dsig.ParseSignature(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if sig2.ID != sig.ID {
		t.Error("ID mismatch after round-trip")
	}
	if sig2.SignatureMethodURI() != sig.SignatureMethodURI() {
		t.Error("SignatureMethod mismatch after round-trip")
	}
	if len(sig2.SignedInfo.References) != len(sig.SignedInfo.References) {
		t.Error("Reference count mismatch after round-trip")
	}
}

func generateTestCert(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}
