package opc

import (
	"crypto/x509"
	"encoding/xml"
	"fmt"
	"path"

	"office/pkg/ooxml/dsig"
	"office/pkg/ooxml/rels"
)

const (
	// Relationship types for digital signatures per OPC.
	relDigitalSignatureOrigin = "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin"
	relDigitalSignature       = "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/signature"
	relDigitalSignatureCert   = "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/certificate"

	// Content types for digital signature parts.
	contentTypeDigitalSignatureOrigin = "application/vnd.openxmlformats-package.digital-signature-origin"
	contentTypeDigitalSignatureXML    = "application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml"
	contentTypeDigitalSignatureCert   = "application/vnd.openxmlformats-package.digital-signature-certificate"

	// Well-known part names.
	signatureOriginPart = "_xmlsignatures/origin.sigs"
)

// SignaturePart represents a digital signature within an OPC package.
// It wraps the raw OPC part together with the parsed XML-DSig structure.
type SignaturePart struct {
	// PartName is the normalized OPC part path (e.g. "_xmlsignatures/sig1.xml").
	PartName string

	// ContentType is the OPC content type for the part.
	ContentType string

	// Raw is the unparsed XML bytes of the signature.
	Raw []byte

	// Signature is the parsed XML-DSig envelope.
	Signature *dsig.Signature
}

// Signer returns the signer's end-entity certificate, if present in KeyInfo.
func (sp *SignaturePart) Signer() *x509.Certificate {
	if sp == nil || sp.Signature == nil {
		return nil
	}
	return sp.Signature.Signer()
}

// Certificates returns all X.509 certificates embedded in the signature.
func (sp *SignaturePart) Certificates() []*x509.Certificate {
	if sp == nil || sp.Signature == nil {
		return nil
	}
	return sp.Signature.Certificates()
}

// SignatureMethod returns the signature algorithm URI.
func (sp *SignaturePart) SignatureMethod() string {
	if sp == nil || sp.Signature == nil {
		return ""
	}
	return sp.Signature.SignatureMethodURI()
}

// HasSignatures reports whether the package contains any digital signature
// origin part, indicating that it is (or was) digitally signed.
func (p *Package) HasSignatures() bool {
	if p == nil {
		return false
	}
	_, ok := p.parts[signatureOriginPart]
	return ok
}

// Signatures discovers and parses all digital signature parts in the package.
// The returned slice is ordered by relationship ID (sig1, sig2, …).
// Individual signatures that fail to parse are skipped so that callers still
// receive any valid signatures.
func (p *Package) Signatures() []SignaturePart {
	if p == nil {
		return nil
	}

	// Locate the signature origin relationship document.
	originRelsName := rels.RelationshipsPartName(signatureOriginPart)
	originRelsPart, ok := p.parts[normalizePartName(originRelsName)]
	if !ok {
		return nil
	}

	originRels, err := rels.Parse(originRelsPart.Data)
	if err != nil {
		return nil
	}

	var result []SignaturePart
	for _, rel := range originRels.Relationships {
		if rel.IsExternal() {
			continue
		}
		if rel.Type != relDigitalSignature && rel.Type != relDigitalSignatureCert {
			continue
		}

		targetName := rels.ResolveTarget(signatureOriginPart, rel.Target)
		part, found := p.parts[normalizePartName(targetName)]
		if !found {
			continue
		}

		sp := SignaturePart{
			PartName:    part.Name,
			ContentType: part.ContentType,
			Raw:         part.Data,
		}

		if rel.Type == relDigitalSignature {
			if sig, err := dsig.ParseSignature(part.Data); err == nil {
				sp.Signature = sig
			}
		}

		result = append(result, sp)
	}

	return result
}

// AddSignaturePart stores a raw signature XML part in the package.
// It updates the signature origin relationships so that the part is
// reachable via standard OPC discovery.
//
// The partName should be relative to the package root, e.g.
// "_xmlsignatures/sig1.xml". If a signature origin part does not yet
// exist, one is created automatically.
func (p *Package) AddSignaturePart(partName string, data []byte) {
	if p == nil {
		return
	}

	name := normalizePartName(partName)
	p.parts[name] = &Part{
		Name:        name,
		ContentType: contentTypeDigitalSignatureXML,
		Data:        data,
	}
	p.contentTypeOverrides[name] = contentTypeDigitalSignatureXML

	// Ensure the signature origin part exists.
	if _, ok := p.parts[signatureOriginPart]; !ok {
		p.parts[signatureOriginPart] = &Part{
			Name:        signatureOriginPart,
			ContentType: contentTypeDigitalSignatureOrigin,
			Data:        []byte{},
		}
		p.contentTypeOverrides[signatureOriginPart] = contentTypeDigitalSignatureOrigin

		// Add origin relationship to root .rels.
		rootRelsName := rels.RelationshipsPartName("")
		rootRelsPart, ok := p.parts[normalizePartName(rootRelsName)]
		if ok {
			if doc, err := rels.Parse(rootRelsPart.Data); err == nil {
				doc.Relationships = append(doc.Relationships, rels.Relationship{
					ID:     "rIdSigOrigin",
					Type:   relDigitalSignatureOrigin,
					Target: signatureOriginPart,
				})
				if marshaled, err := marshalRelationships(doc); err == nil {
					rootRelsPart.Data = marshaled
				}
			}
		}
	}

	// Add relationship from origin to the new signature part.
	originRelsName := rels.RelationshipsPartName(signatureOriginPart)
	originRelsPart, ok := p.parts[normalizePartName(originRelsName)]
	if !ok {
		originRelsPart = &Part{
			Name: normalizePartName(originRelsName),
			Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`),
		}
		p.parts[normalizePartName(originRelsName)] = originRelsPart
	}

	if doc, err := rels.Parse(originRelsPart.Data); err == nil {
		doc.Relationships = append(doc.Relationships, rels.Relationship{
			ID:     nextRID(doc.Relationships, "sig"),
			Type:   relDigitalSignature,
			Target: relTargetForOrigin(name),
		})
		if marshaled, err := marshalRelationships(doc); err == nil {
			originRelsPart.Data = marshaled
		}
	}
}

func relTargetForOrigin(name string) string {
	// Relationship targets are relative to the source part.
	// The origin part lives at _xmlsignatures/origin.sigs, so targets
	// in the same directory are just the filename.
	if path.Dir(name) == "_xmlsignatures" {
		return path.Base(name)
	}
	return name
}

func marshalRelationships(doc *rels.Document) ([]byte, error) {
	type relationships struct {
		XMLName       xml.Name            `xml:"http://schemas.openxmlformats.org/package/2006/relationships Relationships"`
		Relationships []rels.Relationship `xml:"Relationship"`
	}
	return xml.MarshalIndent(relationships{Relationships: doc.Relationships}, "", "  ")
}

func nextRID(rels []rels.Relationship, prefix string) string {
	max := 0
	for _, r := range rels {
		var n int
		if _, err := fmt.Sscanf(r.ID, "rId"+prefix+"%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("rId%s%d", prefix, max+1)
}
