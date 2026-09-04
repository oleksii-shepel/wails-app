package opc_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"office/pkg/ooxml/opc"
)

func buildSignedPackage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// [Content_Types].xml
	ct, _ := zw.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/_xmlsignatures/origin.sigs" ContentType="application/vnd.openxmlformats-package.digital-signature-origin"/>
  <Override PartName="/_xmlsignatures/sig1.xml" ContentType="application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml"/>
</Types>`))

	// _rels/.rels
	rootRels, _ := zw.Create("_rels/.rels")
	rootRels.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rIdSigOrigin" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin" Target="_xmlsignatures/origin.sigs"/>
</Relationships>`))

	// word/document.xml
	doc, _ := zw.Create("word/document.xml")
	doc.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`))

	// _xmlsignatures/origin.sigs (empty part)
	origin, _ := zw.Create("_xmlsignatures/origin.sigs")
	origin.Write([]byte{})

	// _xmlsignatures/_rels/origin.sigs.rels
	originRels, _ := zw.Create("_xmlsignatures/_rels/origin.sigs.rels")
	originRels.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdsig1" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/signature" Target="sig1.xml"/>
</Relationships>`))

	// _xmlsignatures/sig1.xml
	sig, _ := zw.Create("_xmlsignatures/sig1.xml")
	sig.Write([]byte(`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
</Signature>`))

	zw.Close()
	return buf.Bytes()
}

func TestPackageHasSignatures(t *testing.T) {
	data := buildSignedPackage(t)
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !pkg.HasSignatures() {
		t.Error("expected HasSignatures = true")
	}
}

func TestPackageSignatures(t *testing.T) {
	data := buildSignedPackage(t)
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	sigs := pkg.Signatures()
	if len(sigs) != 1 {
		t.Fatalf("Signatures = %d, want 1", len(sigs))
	}

	sp := sigs[0]
	if sp.PartName != "_xmlsignatures/sig1.xml" {
		t.Errorf("PartName = %q", sp.PartName)
	}
	if sp.ContentType != "application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml" {
		t.Errorf("ContentType = %q", sp.ContentType)
	}
	if sp.Signature == nil {
		t.Fatal("Signature = nil")
	}
	if sp.Signature.ID != "sig1" {
		t.Errorf("Signature.ID = %q", sp.Signature.ID)
	}
	if sp.Signer() != nil {
		t.Error("expected nil signer (no KeyInfo in test)")
	}
}

func TestPackageSignaturesEmpty(t *testing.T) {
	pkg := opc.NewPackage()
	if pkg.HasSignatures() {
		t.Error("new package should not have signatures")
	}
	if sigs := pkg.Signatures(); len(sigs) != 0 {
		t.Errorf("Signatures = %d, want 0", len(sigs))
	}
}

func TestAddSignaturePart(t *testing.T) {
	pkg := opc.NewPackage()

	sigXML := []byte(`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="newsig">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
</Signature>`)

	pkg.AddSignaturePart("_xmlsignatures/sig1.xml", sigXML)

	if !pkg.HasSignatures() {
		t.Error("expected HasSignatures = true after add")
	}

	sigs := pkg.Signatures()
	if len(sigs) != 1 {
		t.Fatalf("Signatures = %d, want 1", len(sigs))
	}
	if sigs[0].Signature.ID != "newsig" {
		t.Errorf("ID = %q", sigs[0].Signature.ID)
	}
}
