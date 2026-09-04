package opc

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestVerifySignaturesValidPackage(t *testing.T) {
	data := buildVerifiableSignedPackage(t, signatureFixtureOptions{})

	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	results := pkg.VerifySignatures()
	if len(results) != 1 {
		t.Fatalf("verification count = %d, want 1", len(results))
	}
	result := results[0]
	if !result.Valid {
		t.Fatalf("expected overall valid result, got %+v", result)
	}
	if !result.SignatureValid || !result.SignedInfoValid || !result.PackageReferencesValid {
		t.Fatalf("verification flags = %+v", result)
	}
	if !result.TrustValid || !result.RevocationChecked || result.Revoked {
		t.Fatalf("trust flags = %+v", result)
	}
	if !result.SigningTimeValid || !result.TrustedTimestamp {
		t.Fatalf("timestamp flags = %+v", result)
	}
	if len(result.TrustChain) != 3 {
		t.Fatalf("trust chain length = %d, want 3", len(result.TrustChain))
	}
	if len(result.SignedInfoReferences) != 1 || !result.SignedInfoReferences[0].Valid {
		t.Fatalf("signed info references = %+v", result.SignedInfoReferences)
	}
	if len(result.PackageReferences) != 2 {
		t.Fatalf("package references = %d, want 2", len(result.PackageReferences))
	}
	for _, ref := range result.PackageReferences {
		if !ref.Valid {
			t.Fatalf("expected valid package reference, got %+v", ref)
		}
	}
}

func TestVerifySignaturesDetectsRevokedSignerCertificate(t *testing.T) {
	data := buildVerifiableSignedPackage(t, signatureFixtureOptions{revokeSigner: true})

	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	results := pkg.VerifySignatures()
	if len(results) != 1 {
		t.Fatalf("verification count = %d, want 1", len(results))
	}
	result := results[0]
	if result.Valid {
		t.Fatalf("expected invalid result for revoked signer, got %+v", result)
	}
	if !result.TrustValid || !result.RevocationChecked || !result.Revoked {
		t.Fatalf("revocation flags = %+v", result)
	}
}

func TestVerifySignaturesDetectsTamperedPackagePart(t *testing.T) {
	data := buildVerifiableSignedPackage(t, signatureFixtureOptions{tamperDocument: true})

	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	results := pkg.VerifySignatures()
	if len(results) != 1 {
		t.Fatalf("verification count = %d, want 1", len(results))
	}
	result := results[0]
	if result.Valid {
		t.Fatalf("expected invalid result after package tamper, got %+v", result)
	}
	if !result.SignatureValid || !result.TrustValid {
		t.Fatalf("expected signature and trust to remain valid, got %+v", result)
	}
	if result.PackageReferencesValid {
		t.Fatalf("expected PackageReferencesValid = false, got %+v", result)
	}
}

func TestVerifySignaturesDetectsTamperedSignatureValue(t *testing.T) {
	data := buildVerifiableSignedPackage(t, signatureFixtureOptions{tamperSignature: true})

	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	results := pkg.VerifySignatures()
	if len(results) != 1 {
		t.Fatalf("verification count = %d, want 1", len(results))
	}
	result := results[0]
	if result.Valid {
		t.Fatalf("expected invalid result after signature tamper, got %+v", result)
	}
	if result.SignatureValid {
		t.Fatalf("expected SignatureValid = false, got %+v", result)
	}
	if !result.SignedInfoValid || !result.PackageReferencesValid || !result.TrustValid {
		t.Fatalf("expected references and trust to remain valid, got %+v", result)
	}
}

func TestVerifySignaturesSupportsECDSA(t *testing.T) {
	data := buildVerifiableSignedPackage(t, signatureFixtureOptions{
		signatureMethod: algorithmSignatureECDSASHA256,
	})

	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	results := pkg.VerifySignatures()
	if len(results) != 1 {
		t.Fatalf("verification count = %d, want 1", len(results))
	}
	if !results[0].Valid {
		t.Fatalf("expected ECDSA signature to verify, got %+v", results[0])
	}
}

type signatureFixtureOptions struct {
	tamperDocument    bool
	tamperSignature   bool
	revokeSigner      bool
	signatureMethod   string
	canonicalization  string
	signingTimeLayout string
}

type signerMaterial struct {
	key          crypto.Signer
	signerCert   *x509.Certificate
	interCert    *x509.Certificate
	rootCert     *x509.Certificate
	signerDER    []byte
	interDER     []byte
	rootDER      []byte
	issuerCRLDER []byte
	signingTime  time.Time
}

func buildVerifiableSignedPackage(t *testing.T, opts signatureFixtureOptions) []byte {
	t.Helper()

	const (
		documentContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
		relsContentType     = "application/vnd.openxmlformats-package.relationships+xml"
	)

	if opts.signatureMethod == "" {
		opts.signatureMethod = algorithmSignatureRSASHA256
	}
	if opts.canonicalization == "" {
		opts.canonicalization = algorithmCanonicalXML10
	}
	material := generateSignerMaterial(t, opts)

	documentXML := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`
	documentRelsXML := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`
	documentDigest := mustDigestCanonicalXML(t, documentXML, algorithmDigestSHA256)
	relsDigest := mustDigestRelationshipTransform(t, documentRelsXML, []string{"rIdImage"}, algorithmDigestSHA256)

	signaturePropertiesXML := `<SignatureProperties><SignatureProperty Id="idSignatureTime" Target="#sig1"><mdssi:SignatureTime xmlns:mdssi="` + relationshipTransformNamespace + `"><mdssi:Format>W3CDTF</mdssi:Format><mdssi:Value>` + material.signingTime.Format(time.RFC3339) + `</mdssi:Value></mdssi:SignatureTime></SignatureProperty></SignatureProperties>`
	objectXML := `<Object Id="idPackageObject" xmlns="http://www.w3.org/2000/09/xmldsig#" xmlns:pds="http://schemas.openxmlformats.org/package/2006/digital-signature">` +
		`<Manifest>` +
		`<Reference URI="/word/document.xml?ContentType=` + documentContentType + `">` +
		`<Transforms><Transform Algorithm="` + algorithmCanonicalXML10 + `"/></Transforms>` +
		`<DigestMethod Algorithm="` + algorithmDigestSHA256 + `"/>` +
		`<DigestValue>` + base64.StdEncoding.EncodeToString(documentDigest) + `</DigestValue>` +
		`</Reference>` +
		`<Reference URI="/word/_rels/document.xml.rels?ContentType=` + relsContentType + `">` +
		`<Transforms>` +
		`<Transform Algorithm="` + algorithmRelationshipTransform2006 + `"><pds:RelationshipReference SourceId="rIdImage"/></Transform>` +
		`<Transform Algorithm="` + algorithmCanonicalXML10 + `"/>` +
		`</Transforms>` +
		`<DigestMethod Algorithm="` + algorithmDigestSHA256 + `"/>` +
		`<DigestValue>` + base64.StdEncoding.EncodeToString(relsDigest) + `</DigestValue>` +
		`</Reference>` +
		`</Manifest>` +
		signaturePropertiesXML +
		`</Object>`

	objectDigest := mustDigestObject(t, objectXML, algorithmDigestSHA256)

	signedInfoXML := `<SignedInfo xmlns="http://www.w3.org/2000/09/xmldsig#">` +
		`<CanonicalizationMethod Algorithm="` + opts.canonicalization + `"/>` +
		`<SignatureMethod Algorithm="` + opts.signatureMethod + `"/>` +
		`<Reference URI="#idPackageObject">` +
		`<Transforms><Transform Algorithm="` + algorithmCanonicalXML10 + `"/></Transforms>` +
		`<DigestMethod Algorithm="` + algorithmDigestSHA256 + `"/>` +
		`<DigestValue>` + base64.StdEncoding.EncodeToString(objectDigest) + `</DigestValue>` +
		`</Reference>` +
		`</SignedInfo>`

	keyInfoXML := `<KeyInfo><X509Data>` +
		`<X509Certificate>` + base64.StdEncoding.EncodeToString(material.signerDER) + `</X509Certificate>` +
		`<X509Certificate>` + base64.StdEncoding.EncodeToString(material.interDER) + `</X509Certificate>` +
		`<X509Certificate>` + base64.StdEncoding.EncodeToString(material.rootDER) + `</X509Certificate>` +
		`<X509CRL>` + base64.StdEncoding.EncodeToString(material.issuerCRLDER) + `</X509CRL>` +
		`</X509Data></KeyInfo>`

	signatureTemplate := `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">` +
		signedInfoXML +
		`<SignatureValue></SignatureValue>` +
		keyInfoXML +
		objectXML +
		`</Signature>`

	signatureRoot, err := parseRawXML([]byte(signatureTemplate))
	if err != nil {
		t.Fatalf("parse signature template: %v", err)
	}
	signedInfoNode := firstChildNamed(signatureRoot, "SignedInfo")
	if signedInfoNode == nil {
		t.Fatal("SignedInfo node missing from signature template")
	}
	canonicalSignedInfo, err := canonicalizeSubtree(signedInfoNode, opts.canonicalization)
	if err != nil {
		t.Fatalf("canonicalize SignedInfo: %v", err)
	}
	hashed := hashSignedInfoForTest(t, opts.signatureMethod, canonicalSignedInfo)
	signatureValue := signSignatureValue(t, material.key, opts.signatureMethod, hashed)
	signatureValueB64 := base64.StdEncoding.EncodeToString(signatureValue)
	if opts.tamperSignature {
		signatureValueB64 = mutateBase64(signatureValueB64)
	}

	signatureXML := `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">` +
		signedInfoXML +
		`<SignatureValue>` + signatureValueB64 + `</SignatureValue>` +
		keyInfoXML +
		objectXML +
		`</Signature>`

	if opts.tamperDocument {
		documentXML = strings.Replace(documentXML, "Hello", "Tampered", 1)
	}

	return buildZipPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="` + documentContentType + `"/>
  <Override PartName="/_xmlsignatures/origin.sigs" ContentType="application/vnd.openxmlformats-package.digital-signature-origin"/>
  <Override PartName="/_xmlsignatures/sig1.xml" ContentType="application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rIdSigOrigin" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin" Target="_xmlsignatures/origin.sigs"/>
</Relationships>`,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRelsXML,
		"word/media/image1.png":        "png-data",
		"_xmlsignatures/origin.sigs":   ``,
		"_xmlsignatures/_rels/origin.sigs.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdsig1" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/signature" Target="sig1.xml"/>
</Relationships>`,
		"_xmlsignatures/sig1.xml": signatureXML,
	})
}

func mustDigestObject(t *testing.T, objectXML, digestAlgorithm string) []byte {
	t.Helper()

	root, err := parseRawXML([]byte(`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">` + objectXML + `</Signature>`))
	if err != nil {
		t.Fatalf("parse object wrapper: %v", err)
	}
	objectNode := firstChildNamed(root, "Object")
	if objectNode == nil {
		t.Fatal("Object node missing from wrapper")
	}
	canonical, err := canonicalizeSubtree(objectNode, algorithmCanonicalXML10)
	if err != nil {
		t.Fatalf("canonicalize object: %v", err)
	}
	digest, err := digestBytes(digestAlgorithm, canonical)
	if err != nil {
		t.Fatalf("digest object: %v", err)
	}
	return digest
}

func mustDigestCanonicalXML(t *testing.T, xmlPayload, digestAlgorithm string) []byte {
	t.Helper()

	canonical, err := canonicalizeXMLBytes([]byte(xmlPayload), algorithmCanonicalXML10)
	if err != nil {
		t.Fatalf("canonicalize XML: %v", err)
	}
	digest, err := digestBytes(digestAlgorithm, canonical)
	if err != nil {
		t.Fatalf("digest canonical XML: %v", err)
	}
	return digest
}

func mustDigestRelationshipTransform(t *testing.T, relsXML string, ids []string, digestAlgorithm string) []byte {
	t.Helper()

	transformed, err := applyRelationshipTransform([]byte(relsXML), ids)
	if err != nil {
		t.Fatalf("relationship transform: %v", err)
	}
	canonical, err := canonicalizeXMLBytes(transformed, algorithmCanonicalXML10)
	if err != nil {
		t.Fatalf("canonicalize relationship transform: %v", err)
	}
	digest, err := digestBytes(digestAlgorithm, canonical)
	if err != nil {
		t.Fatalf("digest relationship transform: %v", err)
	}
	return digest
}

func generateSignerMaterial(t *testing.T, opts signatureFixtureOptions) signerMaterial {
	t.Helper()

	signingTime := time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate root key: %v", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root"},
		NotBefore:             signingTime.Add(-48 * time.Hour),
		NotAfter:              signingTime.Add(48 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		SubjectKeyId:          []byte{1, 2, 3, 4},
	}
	rootDER := mustCreateCertificate(t, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	rootCert := mustParseCertificate(t, rootDER)

	interKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate intermediate key: %v", err)
	}
	interTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "test-intermediate"},
		NotBefore:             signingTime.Add(-24 * time.Hour),
		NotAfter:              signingTime.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		SubjectKeyId:          []byte{5, 6, 7, 8},
	}
	interDER := mustCreateCertificate(t, interTemplate, rootCert, &interKey.PublicKey, rootKey)
	interCert := mustParseCertificate(t, interDER)

	leafKey := generateLeafKey(t, opts.signatureMethod)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-signer"},
		NotBefore:    signingTime.Add(-time.Hour),
		NotAfter:     signingTime.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		SubjectKeyId: []byte{9, 10, 11, 12},
	}
	signerDER := mustCreateCertificate(t, leafTemplate, interCert, leafKey.Public(), interKey)
	signerCert := mustParseCertificate(t, signerDER)

	var revokedEntries []x509.RevocationListEntry
	if opts.revokeSigner {
		revokedEntries = append(revokedEntries, x509.RevocationListEntry{
			SerialNumber:   signerCert.SerialNumber,
			RevocationTime: signingTime.Add(-30 * time.Minute),
		})
	}
	crlDER, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                signingTime.Add(-time.Hour),
		NextUpdate:                signingTime.Add(time.Hour),
		RevokedCertificateEntries: revokedEntries,
	}, interCert, interKey)
	if err != nil {
		t.Fatalf("create CRL: %v", err)
	}

	return signerMaterial{
		key:          leafKey,
		signerCert:   signerCert,
		interCert:    interCert,
		rootCert:     rootCert,
		signerDER:    signerDER,
		interDER:     interDER,
		rootDER:      rootDER,
		issuerCRLDER: crlDER,
		signingTime:  signingTime,
	}
}

func generateLeafKey(t *testing.T, signatureMethod string) crypto.Signer {
	t.Helper()

	switch signatureMethod {
	case algorithmSignatureECDSASHA1, algorithmSignatureECDSASHA256, algorithmSignatureECDSASHA384, algorithmSignatureECDSASHA512:
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate ECDSA key: %v", err)
		}
		return key
	default:
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate RSA key: %v", err)
		}
		return key
	}
}

func mustCreateCertificate(t *testing.T, template, parent *x509.Certificate, publicKey any, signer crypto.Signer) []byte {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, signer)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return der
}

func mustParseCertificate(t *testing.T, der []byte) *x509.Certificate {
	t.Helper()

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func signSignatureValue(t *testing.T, signer crypto.Signer, signatureMethod string, digest []byte) []byte {
	t.Helper()

	switch signatureMethod {
	case algorithmSignatureRSASHA1, algorithmSignatureRSASHA256, algorithmSignatureRSASHA384, algorithmSignatureRSASHA512:
		key, ok := signer.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("expected RSA signer for %q, got %T", signatureMethod, signer)
		}
		hashID := hashForSignatureMethodForTest(t, signatureMethod)
		value, err := rsa.SignPKCS1v15(rand.Reader, key, hashID, digest)
		if err != nil {
			t.Fatalf("sign PKCS1v15: %v", err)
		}
		return value
	case algorithmSignatureRSAPSSSHA1, algorithmSignatureRSAPSSSHA256, algorithmSignatureRSAPSSSHA384, algorithmSignatureRSAPSSSHA512:
		key, ok := signer.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("expected RSA signer for %q, got %T", signatureMethod, signer)
		}
		hashID := hashForSignatureMethodForTest(t, signatureMethod)
		value, err := rsa.SignPSS(rand.Reader, key, hashID, digest, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		if err != nil {
			t.Fatalf("sign PSS: %v", err)
		}
		return value
	case algorithmSignatureECDSASHA1, algorithmSignatureECDSASHA256, algorithmSignatureECDSASHA384, algorithmSignatureECDSASHA512:
		key, ok := signer.(*ecdsa.PrivateKey)
		if !ok {
			t.Fatalf("expected ECDSA signer for %q, got %T", signatureMethod, signer)
		}
		value, err := ecdsa.SignASN1(rand.Reader, key, digest)
		if err != nil {
			t.Fatalf("sign ECDSA: %v", err)
		}
		return value
	default:
		t.Fatalf("unsupported signature method %q", signatureMethod)
		return nil
	}
}

func hashForSignatureMethodForTest(t *testing.T, signatureMethod string) crypto.Hash {
	t.Helper()

	spec, err := signatureMethodForURI(signatureMethod)
	if err != nil {
		t.Fatalf("signature method: %v", err)
	}
	return spec.Hash
}

func hashSignedInfoForTest(t *testing.T, signatureMethod string, canonicalSignedInfo []byte) []byte {
	t.Helper()

	hashID := hashForSignatureMethodForTest(t, signatureMethod)
	switch hashID {
	case crypto.SHA1:
		sum := sha1.Sum(canonicalSignedInfo)
		return sum[:]
	case crypto.SHA256:
		sum := sha256.Sum256(canonicalSignedInfo)
		return sum[:]
	case crypto.SHA384:
		sum := sha512.Sum384(canonicalSignedInfo)
		return sum[:]
	case crypto.SHA512:
		sum := sha512.Sum512(canonicalSignedInfo)
		return sum[:]
	default:
		t.Fatalf("unsupported hash %v", hashID)
		return nil
	}
}

func mutateBase64(value string) string {
	if strings.HasSuffix(value, "A") {
		return value[:len(value)-1] + "B"
	}
	return value[:len(value)-1] + "A"
}

func buildZipPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
