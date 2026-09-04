// Package dsig provides Go types for XML-DSig (W3C XML Signature Syntax
// and Processing) structures used by OOXML digital signatures.
//
// It supports parsing the envelope structure, extracting X.509 certificates
// from KeyInfo, and round-tripping the XML. Cryptographic verification is
// implemented by the OPC layer (`pkg/ooxml/opc`); this package intentionally
// stays focused on XML-DSig structure modeling.
package dsig

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
)

const (
	// Namespace is the W3C XML Digital Signature namespace.
	Namespace = "http://www.w3.org/2000/09/xmldsig#"
)

// Signature is the root element of an XML-DSig signature.
type Signature struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	ID      string   `xml:"Id,attr,omitempty"`

	SignedInfo     SignedInfo     `xml:"SignedInfo"`
	SignatureValue SignatureValue `xml:"SignatureValue"`
	KeyInfo        *KeyInfo       `xml:"KeyInfo,omitempty"`
	Objects        []Object       `xml:"Object,omitempty"`
}

// SignedInfo contains the information that is actually signed.
type SignedInfo struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# SignedInfo"`
	ID      string   `xml:"Id,attr,omitempty"`

	CanonicalizationMethod Method      `xml:"CanonicalizationMethod"`
	SignatureMethod        Method      `xml:"SignatureMethod"`
	References             []Reference `xml:"Reference"`
}

// Method captures a canonicalization, signature, or digest method element.
type Method struct {
	XMLName   xml.Name `xml:""`
	Algorithm string   `xml:"Algorithm,attr"`
	Content   []byte   `xml:",innerxml"`
}

// SignatureValue holds the base64-encoded signature bytes.
type SignatureValue struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# SignatureValue"`
	ID      string   `xml:"Id,attr,omitempty"`
	Value   []byte   `xml:",chardata"`
}

// Reference identifies a resource signed by the signature.
type Reference struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Reference"`
	ID      string   `xml:"Id,attr,omitempty"`
	URI     string   `xml:"URI,attr,omitempty"`
	Type    string   `xml:"Type,attr,omitempty"`

	Transforms   *Transforms `xml:"Transforms,omitempty"`
	DigestMethod Method      `xml:"DigestMethod"`
	DigestValue  []byte      `xml:"DigestValue"`
}

// Transforms is a list of transforms applied to a reference.
type Transforms struct {
	XMLName   xml.Name    `xml:"http://www.w3.org/2000/09/xmldsig# Transforms"`
	Transform []Transform `xml:"Transform"`
}

// Transform describes a single transform step.
type Transform struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Transform"`
	Algorithm string   `xml:"Algorithm,attr"`
	Content   []byte   `xml:",innerxml"`
}

// KeyInfo carries key/certificate information about the signature.
type KeyInfo struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	ID      string   `xml:"Id,attr,omitempty"`

	X509Data *X509Data `xml:"X509Data,omitempty"`
	// Raw captures any unrecognized child elements for round-tripping.
	Raw []byte `xml:",innerxml"`
}

// X509Data holds X.509 certificate-related information.
type X509Data struct {
	XMLName      xml.Name          `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
	IssuerSerial *X509IssuerSerial `xml:"X509IssuerSerial,omitempty"`
	SKI          []byte            `xml:"X509SKI,omitempty"`
	SubjectName  string            `xml:"X509SubjectName,omitempty"`
	Certificates [][]byte          `xml:"X509Certificate"`
	CRL          []byte            `xml:"X509CRL,omitempty"`
}

// X509IssuerSerial identifies a certificate by issuer name and serial number.
type X509IssuerSerial struct {
	XMLName      xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# X509IssuerSerial"`
	IssuerName   string   `xml:"X509IssuerName"`
	SerialNumber string   `xml:"X509SerialNumber"`
}

// Object is an optional container for additional signed or unsigned data
// such as XAdES qualifying properties.
type Object struct {
	XMLName  xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Object"`
	ID       string   `xml:"Id,attr,omitempty"`
	MimeType string   `xml:"MimeType,attr,omitempty"`
	Encoding string   `xml:"Encoding,attr,omitempty"`
	Raw      []byte   `xml:",innerxml"`
}

// ParseSignature unmarshals an XML-DSig Signature from XML bytes.
func ParseSignature(data []byte) (*Signature, error) {
	var sig Signature
	if err := xml.Unmarshal(data, &sig); err != nil {
		return nil, err
	}
	return &sig, nil
}

// MarshalXML serialises a Signature to XML bytes.
func (s *Signature) MarshalXML() ([]byte, error) {
	return xml.MarshalIndent(s, "", "  ")
}

// Certificates parses the base64-encoded X.509 certificates in KeyInfo
// and returns them as *x509.Certificate values. Entries that fail to
// parse are silently skipped.
func (s *Signature) Certificates() []*x509.Certificate {
	if s == nil || s.KeyInfo == nil || s.KeyInfo.X509Data == nil {
		return nil
	}
	var certs []*x509.Certificate
	for _, der := range s.KeyInfo.X509Data.Certificates {
		if len(der) == 0 {
			continue
		}
		// The XML may contain base64-encoded data with whitespace.
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(der)))
		n, err := base64.StdEncoding.Decode(decoded, der)
		if err != nil {
			continue
		}
		cert, err := x509.ParseCertificate(decoded[:n])
		if err != nil {
			continue
		}
		certs = append(certs, cert)
	}
	return certs
}

// Signer returns the first successfully parsed certificate from KeyInfo,
// which is conventionally the signer's end-entity certificate.
func (s *Signature) Signer() *x509.Certificate {
	certs := s.Certificates()
	if len(certs) == 0 {
		return nil
	}
	return certs[0]
}

// SignatureMethodURI returns the algorithm URI used for the signature.
func (s *Signature) SignatureMethodURI() string {
	if s == nil {
		return ""
	}
	return s.SignedInfo.SignatureMethod.Algorithm
}

// CanonicalizationMethodURI returns the canonicalization algorithm URI.
func (s *Signature) CanonicalizationMethodURI() string {
	if s == nil {
		return ""
	}
	return s.SignedInfo.CanonicalizationMethod.Algorithm
}
