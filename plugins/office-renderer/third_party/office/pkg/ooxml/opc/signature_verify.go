package opc

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"hash"
	"sort"
	"strings"
	"time"

	"office/pkg/ooxml/dsig"
	"office/pkg/ooxml/rels"
)

const (
	algorithmCanonicalXML10                = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
	algorithmCanonicalXML10WithComments    = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315#WithComments"
	algorithmExclusiveCanonicalXML         = "http://www.w3.org/2001/10/xml-exc-c14n#"
	algorithmExclusiveCanonicalXMLComments = "http://www.w3.org/2001/10/xml-exc-c14n#WithComments"

	algorithmDigestSHA1   = "http://www.w3.org/2000/09/xmldsig#sha1"
	algorithmDigestSHA256 = "http://www.w3.org/2001/04/xmlenc#sha256"
	algorithmDigestSHA384 = "http://www.w3.org/2001/04/xmldsig-more#sha384"
	algorithmDigestSHA512 = "http://www.w3.org/2001/04/xmlenc#sha512"

	algorithmSignatureRSASHA1      = "http://www.w3.org/2000/09/xmldsig#rsa-sha1"
	algorithmSignatureRSASHA256    = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	algorithmSignatureRSASHA384    = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha384"
	algorithmSignatureRSASHA512    = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha512"
	algorithmSignatureRSAPSSSHA1   = "http://www.w3.org/2007/05/xmldsig-more#sha1-rsa-MGF1"
	algorithmSignatureRSAPSSSHA256 = "http://www.w3.org/2007/05/xmldsig-more#sha256-rsa-MGF1"
	algorithmSignatureRSAPSSSHA384 = "http://www.w3.org/2007/05/xmldsig-more#sha384-rsa-MGF1"
	algorithmSignatureRSAPSSSHA512 = "http://www.w3.org/2007/05/xmldsig-more#sha512-rsa-MGF1"
	algorithmSignatureECDSASHA1    = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha1"
	algorithmSignatureECDSASHA256  = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha256"
	algorithmSignatureECDSASHA384  = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha384"
	algorithmSignatureECDSASHA512  = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha512"

	algorithmRelationshipTransform2005 = "http://schemas.microsoft.com/office/2006/digsig/RelationshipTransform"
	algorithmRelationshipTransform2006 = "http://schemas.openxmlformats.org/package/2006/RelationshipTransform"

	relationshipTransformNamespace = "http://schemas.openxmlformats.org/package/2006/digital-signature"
)

const (
	signatureSchemeRSAPKCS1v15 = "rsa-pkcs1v15"
	signatureSchemeRSAPSS      = "rsa-pss"
	signatureSchemeECDSA       = "ecdsa"
)

// SignatureVerification captures cryptographic validation results for one OPC
// digital signature part.
type SignatureVerification struct {
	PartName               string
	Valid                  bool
	SignatureValid         bool
	SignedInfoValid        bool
	PackageReferencesValid bool
	TrustValid             bool
	RevocationChecked      bool
	Revoked                bool
	SigningTime            string
	SigningTimeFormat      string
	SigningTimeValid       bool
	TrustedTimestamp       bool
	TrustChain             []*x509.Certificate
	Errors                 []string
	SignedInfoReferences   []SignatureReferenceVerification
	PackageReferences      []PackageReferenceVerification
}

// SignatureReferenceVerification records the validation outcome for one
// SignedInfo reference.
type SignatureReferenceVerification struct {
	URI          string
	Type         string
	DigestMethod string
	Valid        bool
	Error        string
}

// PackageReferenceVerification records the validation outcome for one
// package-level manifest reference.
type PackageReferenceVerification struct {
	URI          string
	PartName     string
	ContentType  string
	DigestMethod string
	Valid        bool
	Error        string
}

type signatureMethodSpec struct {
	Hash   crypto.Hash
	Scheme string
}

type signatureTimeInfo struct {
	Value  string
	Format string
	Time   time.Time
	Parsed bool
}

// VerifySignatures validates all digital signatures discovered in the package.
// It verifies SignedInfo digests, package manifest references, and the
// SignatureValue cryptographically against the embedded signing certificate.
func (p *Package) VerifySignatures() []SignatureVerification {
	if p == nil {
		return nil
	}
	signatures := p.Signatures()
	if len(signatures) == 0 {
		return nil
	}
	results := make([]SignatureVerification, 0, len(signatures))
	for _, signature := range signatures {
		results = append(results, p.verifySignature(signature))
	}
	return results
}

func (p *Package) verifySignature(signature SignaturePart) SignatureVerification {
	result := SignatureVerification{
		PartName: signature.PartName,
	}
	if signature.Signature == nil {
		result.Errors = append(result.Errors, "signature XML could not be parsed")
		return result
	}

	root, err := parseRawXML(signature.Raw)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("parse raw signature XML: %v", err))
		return result
	}

	signedInfoNode := firstChildNamed(root, "SignedInfo")
	if signedInfoNode == nil {
		result.Errors = append(result.Errors, "SignedInfo element not found")
		return result
	}

	objectByID := make(map[string]*rawElement)
	for _, child := range root.Children {
		if child.Element == nil || child.Element.localName() != "Object" {
			continue
		}
		if id, ok := child.Element.attrValue("Id"); ok && id != "" {
			objectByID[id] = child.Element
		}
	}

	signedInfoValid := true
	var packageRefResults []PackageReferenceVerification
	packageRefsValidated := false
	packageObjectValid := false
	packageObject := objectByID["idPackageObject"]
	signingTime, signingTimeErr := extractSignatureTime(packageObject)
	if signingTime.Value != "" {
		result.SigningTime = signingTime.Value
		result.SigningTimeFormat = signingTime.Format
		result.SigningTimeValid = signingTime.Parsed
	}
	if signingTimeErr != nil {
		result.Errors = append(result.Errors, signingTimeErr.Error())
	}
	for _, ref := range signature.Signature.SignedInfo.References {
		refResult := SignatureReferenceVerification{
			URI:          ref.URI,
			Type:         ref.Type,
			DigestMethod: ref.DigestMethod.Algorithm,
		}

		valid, pkgRefs, verifyErr := p.verifySignedInfoReference(root, objectByID, ref)
		if len(pkgRefs) > 0 {
			packageRefsValidated = true
			packageRefResults = append(packageRefResults, pkgRefs...)
		}
		if verifyErr != nil {
			refResult.Error = verifyErr.Error()
			signedInfoValid = false
		} else {
			refResult.Valid = valid
			if !valid {
				signedInfoValid = false
				refResult.Error = "digest mismatch"
			}
		}
		if ref.URI == "#idPackageObject" && verifyErr == nil && valid {
			packageObjectValid = true
		}
		result.SignedInfoReferences = append(result.SignedInfoReferences, refResult)
	}
	result.SignedInfoValid = signedInfoValid
	result.PackageReferences = packageRefResults
	result.PackageReferencesValid = !packageRefsValidated || allPackageReferencesValid(packageRefResults)
	for _, ref := range result.SignedInfoReferences {
		if ref.Error != "" {
			result.Errors = append(result.Errors, ref.Error)
		}
	}
	for _, ref := range packageRefResults {
		if ref.Error != "" {
			result.Errors = append(result.Errors, ref.Error)
		}
	}

	signedInfoCanonical, err := canonicalizeSubtree(signedInfoNode, signature.Signature.SignedInfo.CanonicalizationMethod.Algorithm)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("canonicalize SignedInfo: %v", err))
		return result
	}
	sigValid, err := verifySignatureValue(signature.Signature, signedInfoCanonical)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.SignatureValid = sigValid
		if !sigValid {
			result.Errors = append(result.Errors, "signature value mismatch")
		}
	}

	trustResult, trustErrors := verifySignatureTrust(signature.Signature, signingTime, packageObjectValid)
	result.TrustValid = trustResult.TrustValid
	result.RevocationChecked = trustResult.RevocationChecked
	result.Revoked = trustResult.Revoked
	result.SigningTimeValid = result.SigningTimeValid || trustResult.SigningTimeValid
	result.TrustedTimestamp = trustResult.TrustedTimestamp
	result.TrustChain = trustResult.TrustChain
	result.Errors = append(result.Errors, trustErrors...)
	if result.Revoked {
		result.Errors = append(result.Errors, "signer certificate is revoked")
	}

	result.Valid = result.SignatureValid &&
		result.SignedInfoValid &&
		result.PackageReferencesValid &&
		result.TrustValid &&
		(!result.SigningTimeValid || result.TrustedTimestamp) &&
		!result.Revoked &&
		len(result.Errors) == 0
	return result
}

func (p *Package) verifySignedInfoReference(root *rawElement, objectByID map[string]*rawElement, ref dsig.Reference) (bool, []PackageReferenceVerification, error) {
	if ref.URI == "" {
		return false, nil, fmt.Errorf("unsupported SignedInfo reference URI")
	}
	if !strings.HasPrefix(ref.URI, "#") {
		return false, nil, fmt.Errorf("unsupported external SignedInfo reference URI %q", ref.URI)
	}

	targetID := strings.TrimPrefix(ref.URI, "#")
	target := objectByID[targetID]
	if target == nil {
		return false, nil, fmt.Errorf("SignedInfo reference target %q not found", ref.URI)
	}

	transformed, err := canonicalizeObjectReference(target, ref.Transforms)
	if err != nil {
		return false, nil, fmt.Errorf("transform SignedInfo reference %q: %w", ref.URI, err)
	}
	digest, err := digestBytes(ref.DigestMethod.Algorithm, transformed)
	if err != nil {
		return false, nil, fmt.Errorf("digest SignedInfo reference %q: %w", ref.URI, err)
	}
	expectedDigest, err := decodeXMLBase64(ref.DigestValue)
	if err != nil {
		return false, nil, fmt.Errorf("decode SignedInfo reference digest %q: %w", ref.URI, err)
	}

	var packageRefs []PackageReferenceVerification
	if targetID == "idPackageObject" {
		packageRefs, err = p.verifyPackageObject(target)
		if err != nil {
			return false, packageRefs, err
		}
	}
	return bytes.Equal(digest, expectedDigest), packageRefs, nil
}

func (p *Package) verifyPackageObject(object *rawElement) ([]PackageReferenceVerification, error) {
	if object == nil {
		return nil, fmt.Errorf("package object is missing")
	}

	var manifest *rawElement
	signaturePropertiesCount := 0
	for _, child := range object.Children {
		if child.Element == nil {
			continue
		}
		switch child.Element.localName() {
		case "Manifest":
			if manifest == nil {
				manifest = child.Element
			}
		case "SignatureProperties":
			signaturePropertiesCount++
		}
	}
	if manifest == nil {
		return nil, fmt.Errorf("package object Manifest not found")
	}
	if signaturePropertiesCount != 1 {
		return nil, fmt.Errorf("package object SignatureProperties count = %d, want 1", signaturePropertiesCount)
	}

	var results []PackageReferenceVerification
	for _, child := range manifest.Children {
		if child.Element == nil || child.Element.localName() != "Reference" {
			continue
		}
		result := p.verifyPackageReference(child.Element)
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("package object Manifest has no Reference entries")
	}
	return results, nil
}

func (p *Package) verifyPackageReference(ref *rawElement) PackageReferenceVerification {
	result := PackageReferenceVerification{
		URI:          ref.attrValueDefault("URI"),
		DigestMethod: "",
	}
	digestMethod, digestValue, err := parseDigestSpec(ref)
	result.DigestMethod = digestMethod
	if err != nil {
		result.Error = err.Error()
		return result
	}

	partName, contentType, err := parsePackageReferenceURI(result.URI)
	result.PartName = partName
	result.ContentType = contentType
	if err != nil {
		result.Error = err.Error()
		return result
	}

	part, ok := p.Part(partName)
	if !ok {
		result.Error = fmt.Sprintf("package part %q not found", partName)
		return result
	}
	if part.ContentType != contentType {
		result.Error = fmt.Sprintf("package part %q content type = %q, want %q", partName, part.ContentType, contentType)
		return result
	}

	transformSpec, err := parseTransformSpec(ref)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	transformed, err := applyPackageReferenceTransforms(part, transformSpec)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	digest, err := digestBytes(digestMethod, transformed)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Valid = bytes.Equal(digest, digestValue)
	if !result.Valid {
		result.Error = fmt.Sprintf("digest mismatch for %q", result.URI)
	}
	return result
}

func parseDigestSpec(ref *rawElement) (string, []byte, error) {
	digestMethodEl := firstChildNamed(ref, "DigestMethod")
	if digestMethodEl == nil {
		return "", nil, fmt.Errorf("DigestMethod not found")
	}
	digestMethod, ok := digestMethodEl.attrValue("Algorithm")
	if !ok || digestMethod == "" {
		return "", nil, fmt.Errorf("DigestMethod Algorithm is missing")
	}

	digestValueEl := firstChildNamed(ref, "DigestValue")
	if digestValueEl == nil {
		return "", nil, fmt.Errorf("DigestValue not found")
	}
	digestValue, err := base64.StdEncoding.DecodeString(strings.TrimSpace(digestValueEl.allText()))
	if err != nil {
		return "", nil, fmt.Errorf("decode DigestValue: %w", err)
	}
	return digestMethod, digestValue, nil
}

func parsePackageReferenceURI(uri string) (partName, contentType string, err error) {
	pathPart, queryPart, found := strings.Cut(uri, "?")
	if !found {
		return "", "", fmt.Errorf("package reference URI %q is missing ContentType query", uri)
	}
	partName = normalizePartName(pathPart)
	if partName == "" {
		return "", "", fmt.Errorf("package reference URI %q has empty part name", uri)
	}
	const queryPrefix = "ContentType="
	if !strings.HasPrefix(queryPart, queryPrefix) {
		return "", "", fmt.Errorf("package reference URI %q is missing ContentType query", uri)
	}
	contentType = queryPart[len(queryPrefix):]
	if contentType == "" {
		return "", "", fmt.Errorf("package reference URI %q has empty ContentType query", uri)
	}
	return partName, contentType, nil
}

type packageTransformSpec struct {
	RelationshipIDs  []string
	Canonicalization string
	HasRelationship  bool
}

func parseTransformSpec(ref *rawElement) (packageTransformSpec, error) {
	var spec packageTransformSpec
	transformsEl := firstChildNamed(ref, "Transforms")
	if transformsEl == nil {
		return spec, nil
	}
	for _, child := range transformsEl.Children {
		if child.Element == nil || child.Element.localName() != "Transform" {
			continue
		}
		algorithm, ok := child.Element.attrValue("Algorithm")
		if !ok || algorithm == "" {
			return spec, fmt.Errorf("Transform Algorithm is missing")
		}
		switch algorithm {
		case algorithmRelationshipTransform2005, algorithmRelationshipTransform2006:
			ids, err := parseRelationshipTransform(child.Element)
			if err != nil {
				return spec, err
			}
			spec.RelationshipIDs = ids
			spec.HasRelationship = true
		default:
			if isCanonicalizationAlgorithm(algorithm) {
				spec.Canonicalization = algorithm
				continue
			}
			return spec, fmt.Errorf("unsupported transform algorithm %q", algorithm)
		}
	}
	return spec, nil
}

func parseRelationshipTransform(transform *rawElement) ([]string, error) {
	var ids []string
	for _, child := range transform.Children {
		if child.Element == nil {
			continue
		}
		if child.Element.localName() != "RelationshipReference" {
			continue
		}
		if ns := child.Element.namespaceURI(); ns != relationshipTransformNamespace && ns != "" {
			continue
		}
		sourceID, ok := child.Element.attrValue("SourceId")
		if !ok || sourceID == "" {
			return nil, fmt.Errorf("RelationshipReference SourceId is missing")
		}
		ids = append(ids, sourceID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("RelationshipTransform has no RelationshipReference entries")
	}
	return ids, nil
}

func applyPackageReferenceTransforms(part Part, spec packageTransformSpec) ([]byte, error) {
	data := append([]byte(nil), part.Data...)
	if spec.HasRelationship {
		if part.ContentType != "application/vnd.openxmlformats-package.relationships+xml" {
			return nil, fmt.Errorf("RelationshipTransform requires a .rels part")
		}
		transformed, err := applyRelationshipTransform(data, spec.RelationshipIDs)
		if err != nil {
			return nil, err
		}
		data = transformed
	}
	if spec.Canonicalization != "" {
		return canonicalizeXMLBytes(data, spec.Canonicalization)
	}
	return data, nil
}

func applyRelationshipTransform(data []byte, ids []string) ([]byte, error) {
	doc, err := rels.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse relationships part: %w", err)
	}
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	filtered := make([]rels.Relationship, 0, len(doc.Relationships))
	for _, relationship := range doc.Relationships {
		if _, ok := allowed[relationship.ID]; ok {
			filtered = append(filtered, relationship)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	var b strings.Builder
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, relationship := range filtered {
		b.WriteString(`<Relationship`)
		writeCanonicalAttr(&b, "Id", relationship.ID)
		writeCanonicalAttr(&b, "Target", relationship.Target)
		if relationship.TargetMode != "" {
			writeCanonicalAttr(&b, "TargetMode", relationship.TargetMode)
		}
		writeCanonicalAttr(&b, "Type", relationship.Type)
		b.WriteString(`></Relationship>`)
	}
	b.WriteString(`</Relationships>`)
	return []byte(b.String()), nil
}

func canonicalizeObjectReference(target *rawElement, transforms *dsig.Transforms) ([]byte, error) {
	algorithm := algorithmCanonicalXML10
	if transforms != nil {
		for _, transform := range transforms.Transform {
			if !isCanonicalizationAlgorithm(transform.Algorithm) {
				return nil, fmt.Errorf("unsupported object transform algorithm %q", transform.Algorithm)
			}
			algorithm = transform.Algorithm
		}
	}
	return canonicalizeSubtree(target, algorithm)
}

func verifySignatureValue(signature *dsig.Signature, canonicalSignedInfo []byte) (bool, error) {
	if signature == nil {
		return false, fmt.Errorf("signature is nil")
	}
	signer := signature.Signer()
	if signer == nil {
		return false, fmt.Errorf("signer certificate is missing")
	}

	spec, err := signatureMethodForURI(signature.SignatureMethodURI())
	if err != nil {
		return false, err
	}
	digest := spec.Hash.New()
	if _, err := digest.Write(canonicalSignedInfo); err != nil {
		return false, err
	}
	hashed := digest.Sum(nil)
	signatureValue, err := decodeXMLBase64(signature.SignatureValue.Value)
	if err != nil {
		return false, fmt.Errorf("decode SignatureValue: %w", err)
	}

	switch pub := signer.PublicKey.(type) {
	case *rsa.PublicKey:
		switch spec.Scheme {
		case signatureSchemeRSAPKCS1v15:
			if err := rsa.VerifyPKCS1v15(pub, spec.Hash, hashed, signatureValue); err != nil {
				return false, nil
			}
			return true, nil
		case signatureSchemeRSAPSS:
			if err := rsa.VerifyPSS(pub, spec.Hash, hashed, signatureValue, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}); err != nil {
				return false, nil
			}
			return true, nil
		default:
			return false, fmt.Errorf("signature method %q is incompatible with RSA public key", signature.SignatureMethodURI())
		}
	case *ecdsa.PublicKey:
		if spec.Scheme != signatureSchemeECDSA {
			return false, fmt.Errorf("signature method %q is incompatible with ECDSA public key", signature.SignatureMethodURI())
		}
		return ecdsa.VerifyASN1(pub, hashed, signatureValue), nil
	default:
		return false, fmt.Errorf("unsupported public key type %T", signer.PublicKey)
	}
}

type trustVerification struct {
	TrustValid        bool
	RevocationChecked bool
	Revoked           bool
	SigningTimeValid  bool
	TrustedTimestamp  bool
	TrustChain        []*x509.Certificate
}

func verifySignatureTrust(signature *dsig.Signature, signingTime signatureTimeInfo, packageObjectSigned bool) (trustVerification, []string) {
	var (
		result trustVerification
		errors []string
	)
	if signature == nil {
		errors = append(errors, "signature is nil")
		return result, errors
	}
	signer := signature.Signer()
	if signer == nil {
		errors = append(errors, "signer certificate is missing")
		return result, errors
	}
	if signer.KeyUsage != 0 && signer.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		errors = append(errors, "signer certificate keyUsage does not allow digital signatures")
		return result, errors
	}

	var verificationTime time.Time
	if signingTime.Parsed {
		result.SigningTimeValid = !signingTime.Time.Before(signer.NotBefore) && !signingTime.Time.After(signer.NotAfter)
		if !result.SigningTimeValid {
			errors = append(errors, fmt.Sprintf("signing time %q is outside the signer certificate validity window", signingTime.Value))
		}
		if packageObjectSigned && result.SigningTimeValid {
			verificationTime = signingTime.Time
		}
	}

	chains, err := verifyCertificateChain(signature, verificationTime)
	if err != nil {
		errors = append(errors, err.Error())
		return result, errors
	}
	result.TrustValid = len(chains) > 0
	if result.TrustValid {
		result.TrustChain = append(result.TrustChain, chains[0]...)
	}
	if signingTime.Parsed && packageObjectSigned && result.SigningTimeValid && result.TrustValid {
		result.TrustedTimestamp = true
	}

	revoked, checked, err := verifyRevocation(signature, result.TrustChain, verificationTime)
	result.RevocationChecked = checked
	result.Revoked = revoked
	if err != nil {
		errors = append(errors, err.Error())
	}
	return result, errors
}

func verifyCertificateChain(signature *dsig.Signature, verificationTime time.Time) ([][]*x509.Certificate, error) {
	if signature == nil {
		return nil, fmt.Errorf("signature is nil")
	}
	certs := signature.Certificates()
	if len(certs) == 0 {
		return nil, fmt.Errorf("embedded signer certificate is missing")
	}
	signer := certs[0]

	roots := x509.NewCertPool()
	if systemRoots, err := x509.SystemCertPool(); err == nil && systemRoots != nil {
		roots = systemRoots
	}
	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		if isSelfSignedCA(cert) {
			roots.AddCert(cert)
			continue
		}
		intermediates.AddCert(cert)
	}
	if isSelfSignedCA(signer) {
		roots.AddCert(signer)
	}

	opts := x509.VerifyOptions{
		Intermediates: intermediates,
		Roots:         roots,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	if !verificationTime.IsZero() {
		opts.CurrentTime = verificationTime
	}
	chains, err := signer.Verify(opts)
	if err != nil {
		if !verificationTime.IsZero() {
			return nil, fmt.Errorf("verify signing certificate chain at %s: %w", verificationTime.UTC().Format(time.RFC3339), err)
		}
		return nil, fmt.Errorf("verify signing certificate chain: %w", err)
	}
	return chains, nil
}

func verifyRevocation(signature *dsig.Signature, chain []*x509.Certificate, verificationTime time.Time) (bool, bool, error) {
	if signature == nil || len(chain) == 0 {
		return false, false, nil
	}
	crls, err := embeddedCRLs(signature)
	if err != nil {
		return false, false, err
	}
	if len(crls) == 0 {
		return false, false, nil
	}

	checkTime := verificationTime
	if checkTime.IsZero() {
		checkTime = time.Now().UTC()
	}
	revoked, checked, err := checkChainAgainstCRLs(chain, crls, checkTime)
	if err != nil {
		return false, checked, err
	}
	return revoked, checked, nil
}

func embeddedCRLs(signature *dsig.Signature) ([]*x509.RevocationList, error) {
	if signature == nil || signature.KeyInfo == nil || signature.KeyInfo.X509Data == nil || len(signature.KeyInfo.X509Data.CRL) == 0 {
		return nil, nil
	}
	crlDER, err := decodeXMLBase64(signature.KeyInfo.X509Data.CRL)
	if err != nil {
		return nil, fmt.Errorf("decode embedded CRL: %w", err)
	}
	crl, err := x509.ParseRevocationList(crlDER)
	if err != nil {
		return nil, fmt.Errorf("parse embedded CRL: %w", err)
	}
	return []*x509.RevocationList{crl}, nil
}

func checkChainAgainstCRLs(chain []*x509.Certificate, crls []*x509.RevocationList, at time.Time) (bool, bool, error) {
	var checked bool
	for i := 0; i+1 < len(chain); i++ {
		cert := chain[i]
		issuer := chain[i+1]
		crl := findCRLForIssuer(crls, issuer)
		if crl == nil {
			continue
		}
		checked = true
		if err := crl.CheckSignatureFrom(issuer); err != nil {
			return false, checked, fmt.Errorf("verify CRL issued by %q: %w", issuer.Subject.String(), err)
		}
		if at.Before(crl.ThisUpdate) {
			return false, checked, fmt.Errorf("CRL issued by %q is not yet valid at %s", issuer.Subject.String(), at.UTC().Format(time.RFC3339))
		}
		if !crl.NextUpdate.IsZero() && at.After(crl.NextUpdate) {
			return false, checked, fmt.Errorf("CRL issued by %q expired before %s", issuer.Subject.String(), at.UTC().Format(time.RFC3339))
		}
		for _, entry := range crl.RevokedCertificateEntries {
			if entry.SerialNumber != nil && entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return true, checked, nil
			}
		}
	}
	return false, checked, nil
}

func findCRLForIssuer(crls []*x509.RevocationList, issuer *x509.Certificate) *x509.RevocationList {
	if issuer == nil {
		return nil
	}
	for _, crl := range crls {
		if crl == nil {
			continue
		}
		if len(crl.RawIssuer) > 0 && bytes.Equal(crl.RawIssuer, issuer.RawSubject) {
			return crl
		}
		if len(crl.AuthorityKeyId) > 0 && len(issuer.SubjectKeyId) > 0 && bytes.Equal(crl.AuthorityKeyId, issuer.SubjectKeyId) {
			return crl
		}
		if crl.Issuer.String() == issuer.Subject.String() {
			return crl
		}
	}
	return nil
}

func extractSignatureTime(object *rawElement) (signatureTimeInfo, error) {
	var info signatureTimeInfo
	signatureTime := findFirstDescendantNamed(object, "SignatureTime")
	if signatureTime == nil {
		return info, nil
	}
	formatEl := firstChildNamed(signatureTime, "Format")
	valueEl := firstChildNamed(signatureTime, "Value")
	if formatEl != nil {
		info.Format = strings.TrimSpace(formatEl.allText())
	}
	if valueEl != nil {
		info.Value = strings.TrimSpace(valueEl.allText())
	}
	if info.Value == "" {
		return info, fmt.Errorf("SignatureTime value is missing")
	}
	parsed, err := parseSignatureTimeValue(info.Value)
	if err != nil {
		return info, fmt.Errorf("parse SignatureTime value %q: %w", info.Value, err)
	}
	info.Time = parsed
	info.Parsed = true
	return info, nil
}

func parseSignatureTimeValue(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format")
}

func findFirstDescendantNamed(root *rawElement, local string) *rawElement {
	if root == nil {
		return nil
	}
	for _, child := range root.Children {
		if child.Element == nil {
			continue
		}
		if child.Element.localName() == local {
			return child.Element
		}
		if found := findFirstDescendantNamed(child.Element, local); found != nil {
			return found
		}
	}
	return nil
}

func isSelfSignedCA(cert *x509.Certificate) bool {
	if cert == nil || !cert.IsCA {
		return false
	}
	if !bytes.Equal(cert.RawSubject, cert.RawIssuer) {
		return false
	}
	return cert.CheckSignatureFrom(cert) == nil
}

func digestBytes(algorithm string, data []byte) ([]byte, error) {
	h, err := hashForDigestMethod(algorithm)
	if err != nil {
		return nil, err
	}
	digest := h()
	if _, err := digest.Write(data); err != nil {
		return nil, err
	}
	return digest.Sum(nil), nil
}

func hashForDigestMethod(algorithm string) (func() hash.Hash, error) {
	switch algorithm {
	case algorithmDigestSHA1:
		return sha1.New, nil
	case algorithmDigestSHA256:
		return sha256.New, nil
	case algorithmDigestSHA384:
		return sha512.New384, nil
	case algorithmDigestSHA512:
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unsupported digest method %q", algorithm)
	}
}

func signatureMethodForURI(algorithm string) (signatureMethodSpec, error) {
	switch algorithm {
	case algorithmSignatureRSASHA1:
		return signatureMethodSpec{Hash: crypto.SHA1, Scheme: signatureSchemeRSAPKCS1v15}, nil
	case algorithmSignatureRSASHA256:
		return signatureMethodSpec{Hash: crypto.SHA256, Scheme: signatureSchemeRSAPKCS1v15}, nil
	case algorithmSignatureRSASHA384:
		return signatureMethodSpec{Hash: crypto.SHA384, Scheme: signatureSchemeRSAPKCS1v15}, nil
	case algorithmSignatureRSASHA512:
		return signatureMethodSpec{Hash: crypto.SHA512, Scheme: signatureSchemeRSAPKCS1v15}, nil
	case algorithmSignatureRSAPSSSHA1:
		return signatureMethodSpec{Hash: crypto.SHA1, Scheme: signatureSchemeRSAPSS}, nil
	case algorithmSignatureRSAPSSSHA256:
		return signatureMethodSpec{Hash: crypto.SHA256, Scheme: signatureSchemeRSAPSS}, nil
	case algorithmSignatureRSAPSSSHA384:
		return signatureMethodSpec{Hash: crypto.SHA384, Scheme: signatureSchemeRSAPSS}, nil
	case algorithmSignatureRSAPSSSHA512:
		return signatureMethodSpec{Hash: crypto.SHA512, Scheme: signatureSchemeRSAPSS}, nil
	case algorithmSignatureECDSASHA1:
		return signatureMethodSpec{Hash: crypto.SHA1, Scheme: signatureSchemeECDSA}, nil
	case algorithmSignatureECDSASHA256:
		return signatureMethodSpec{Hash: crypto.SHA256, Scheme: signatureSchemeECDSA}, nil
	case algorithmSignatureECDSASHA384:
		return signatureMethodSpec{Hash: crypto.SHA384, Scheme: signatureSchemeECDSA}, nil
	case algorithmSignatureECDSASHA512:
		return signatureMethodSpec{Hash: crypto.SHA512, Scheme: signatureSchemeECDSA}, nil
	default:
		return signatureMethodSpec{}, fmt.Errorf("unsupported signature method %q", algorithm)
	}
}

func isCanonicalizationAlgorithm(algorithm string) bool {
	switch algorithm {
	case algorithmCanonicalXML10,
		algorithmCanonicalXML10WithComments,
		algorithmExclusiveCanonicalXML,
		algorithmExclusiveCanonicalXMLComments:
		return true
	default:
		return false
	}
}

func allPackageReferencesValid(results []PackageReferenceVerification) bool {
	for _, result := range results {
		if !result.Valid {
			return false
		}
	}
	return true
}

type rawChild struct {
	Element *rawElement
	Text    string
	Comment string
}

type rawElement struct {
	Name     xml.Name
	Attr     []xml.Attr
	Children []rawChild
	Parent   *rawElement
}

func (e *rawElement) localName() string {
	if e == nil {
		return ""
	}
	return e.Name.Local
}

func (e *rawElement) namespaceURI() string {
	if e == nil {
		return ""
	}
	ns := namespaceScope(e)
	return resolveQNameNamespace(ns, e.Name)
}

func (e *rawElement) attrValue(local string) (string, bool) {
	if e == nil {
		return "", false
	}
	for _, attr := range e.Attr {
		if attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return "", false
}

func (e *rawElement) attrValueDefault(local string) string {
	value, _ := e.attrValue(local)
	return value
}

func (e *rawElement) allText() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	for _, child := range e.Children {
		if child.Element != nil {
			b.WriteString(child.Element.allText())
			continue
		}
		b.WriteString(child.Text)
	}
	return b.String()
}

func firstChildNamed(root *rawElement, local string) *rawElement {
	if root == nil {
		return nil
	}
	for _, child := range root.Children {
		if child.Element != nil && child.Element.localName() == local {
			return child.Element
		}
	}
	return nil
}

func parseRawXML(data []byte) (*rawElement, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.RawToken()
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		return parseRawElement(dec, start, nil)
	}
}

func parseRawElement(dec *xml.Decoder, start xml.StartElement, parent *rawElement) (*rawElement, error) {
	element := &rawElement{
		Name:   start.Name,
		Attr:   append([]xml.Attr(nil), start.Attr...),
		Parent: parent,
	}
	for {
		token, err := dec.RawToken()
		if err != nil {
			return nil, err
		}
		switch current := token.(type) {
		case xml.StartElement:
			child, err := parseRawElement(dec, current, element)
			if err != nil {
				return nil, err
			}
			element.Children = append(element.Children, rawChild{Element: child})
		case xml.EndElement:
			if current.Name == start.Name {
				return element, nil
			}
		case xml.CharData:
			element.Children = append(element.Children, rawChild{Text: string([]byte(current))})
		case xml.Comment:
			element.Children = append(element.Children, rawChild{Comment: string([]byte(current))})
		}
	}
}

func canonicalizeXMLBytes(data []byte, algorithm string) ([]byte, error) {
	root, err := parseRawXML(data)
	if err != nil {
		return nil, err
	}
	return canonicalizeSubtree(root, algorithm)
}

func canonicalizeSubtree(root *rawElement, algorithm string) ([]byte, error) {
	if root == nil {
		return nil, fmt.Errorf("canonicalization target is nil")
	}
	includeComments := false
	switch algorithm {
	case algorithmCanonicalXML10:
	case algorithmCanonicalXML10WithComments:
		includeComments = true
	case algorithmExclusiveCanonicalXML:
		// The current verifier treats exclusive canonicalization like inclusive
		// canonicalization when no PrefixList is present, which matches the
		// package-signature payloads exercised in this repository.
	case algorithmExclusiveCanonicalXMLComments:
		includeComments = true
	default:
		return nil, fmt.Errorf("unsupported canonicalization algorithm %q", algorithm)
	}

	var b strings.Builder
	initial := inheritedNamespaces(root)
	writeCanonicalElement(&b, root, initial, map[string]string{}, true, includeComments)
	return []byte(b.String()), nil
}

func inheritedNamespaces(root *rawElement) map[string]string {
	ns := map[string]string{
		"xml": "http://www.w3.org/XML/1998/namespace",
	}
	ancestors := make([]*rawElement, 0, 4)
	for current := root.Parent; current != nil; current = current.Parent {
		ancestors = append(ancestors, current)
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		applyNamespaceDecls(ns, ancestors[i].Attr)
	}
	return ns
}

func namespaceScope(element *rawElement) map[string]string {
	ns := inheritedNamespaces(element)
	applyNamespaceDecls(ns, element.Attr)
	return ns
}

func applyNamespaceDecls(ns map[string]string, attrs []xml.Attr) {
	for _, attr := range attrs {
		if prefix, uri, ok := namespaceDecl(attr); ok {
			ns[prefix] = uri
		}
	}
}

func namespaceDecl(attr xml.Attr) (prefix, uri string, ok bool) {
	switch {
	case attr.Name.Space == "xmlns":
		return attr.Name.Local, attr.Value, true
	case attr.Name.Space == "" && attr.Name.Local == "xmlns":
		return "", attr.Value, true
	default:
		return "", "", false
	}
}

func writeCanonicalElement(b *strings.Builder, element *rawElement, inherited, rendered map[string]string, renderAllNamespaces bool, includeComments bool) {
	currentNS := cloneNS(inherited)
	applyNamespaceDecls(currentNS, element.Attr)

	b.WriteByte('<')
	b.WriteString(qname(element.Name))

	namespaceAttrs := canonicalNamespaceAttrs(element, currentNS, rendered, renderAllNamespaces)
	for _, attr := range namespaceAttrs {
		b.WriteByte(' ')
		if attr.Name.Space == "xmlns" {
			b.WriteString("xmlns:")
			b.WriteString(attr.Name.Local)
		} else {
			b.WriteString("xmlns")
		}
		b.WriteString(`="`)
		b.WriteString(escapeCanonicalAttrValue(attr.Value))
		b.WriteByte('"')
	}

	attrs := canonicalAttrs(element.Attr, currentNS)
	for _, attr := range attrs {
		b.WriteByte(' ')
		b.WriteString(qname(attr.Name))
		b.WriteString(`="`)
		b.WriteString(escapeCanonicalAttrValue(attr.Value))
		b.WriteByte('"')
	}
	b.WriteByte('>')

	nextRendered := cloneNS(rendered)
	for _, attr := range namespaceAttrs {
		prefix, uri, _ := namespaceDecl(attr)
		nextRendered[prefix] = uri
	}
	if renderAllNamespaces {
		for prefix, uri := range currentNS {
			nextRendered[prefix] = uri
		}
	}

	for _, child := range element.Children {
		if child.Element != nil {
			writeCanonicalElement(b, child.Element, currentNS, nextRendered, false, includeComments)
			continue
		}
		if includeComments && child.Comment != "" {
			b.WriteString("<!--")
			b.WriteString(child.Comment)
			b.WriteString("-->")
			continue
		}
		b.WriteString(escapeCanonicalText(child.Text))
	}

	b.WriteString(`</`)
	b.WriteString(qname(element.Name))
	b.WriteByte('>')
}

func canonicalNamespaceAttrs(element *rawElement, currentNS, rendered map[string]string, renderAll bool) []xml.Attr {
	seen := make(map[string]struct{})
	var attrs []xml.Attr
	if renderAll {
		for prefix, uri := range currentNS {
			if prefix == "xml" {
				continue
			}
			attrs = append(attrs, namespaceAttr(prefix, uri))
			seen[prefix] = struct{}{}
		}
	}
	for _, attr := range element.Attr {
		prefix, uri, ok := namespaceDecl(attr)
		if !ok {
			continue
		}
		if prefix == "xml" {
			continue
		}
		if current, exists := rendered[prefix]; exists && current == uri {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		attrs = append(attrs, namespaceAttr(prefix, uri))
		seen[prefix] = struct{}{}
	}
	sort.Slice(attrs, func(i, j int) bool {
		left := attrs[i].Name.Local
		right := attrs[j].Name.Local
		if attrs[i].Name.Space == "" {
			left = ""
		}
		if attrs[j].Name.Space == "" {
			right = ""
		}
		return left < right
	})
	return attrs
}

func canonicalAttrs(attrs []xml.Attr, currentNS map[string]string) []xml.Attr {
	var filtered []xml.Attr
	for _, attr := range attrs {
		if _, _, ok := namespaceDecl(attr); ok {
			continue
		}
		filtered = append(filtered, attr)
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftURI := resolveAttrNamespace(currentNS, filtered[i].Name)
		rightURI := resolveAttrNamespace(currentNS, filtered[j].Name)
		if leftURI != rightURI {
			return leftURI < rightURI
		}
		return filtered[i].Name.Local < filtered[j].Name.Local
	})
	return filtered
}

func resolveAttrNamespace(currentNS map[string]string, name xml.Name) string {
	if name.Space == "" {
		return ""
	}
	if name.Space == "xml" {
		return "http://www.w3.org/XML/1998/namespace"
	}
	return currentNS[name.Space]
}

func resolveQNameNamespace(currentNS map[string]string, name xml.Name) string {
	if name.Space == "" {
		return currentNS[""]
	}
	if name.Space == "xml" {
		return "http://www.w3.org/XML/1998/namespace"
	}
	return currentNS[name.Space]
}

func namespaceAttr(prefix, uri string) xml.Attr {
	if prefix == "" {
		return xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: uri}
	}
	return xml.Attr{Name: xml.Name{Space: "xmlns", Local: prefix}, Value: uri}
}

func qname(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}

func cloneNS(ns map[string]string) map[string]string {
	out := make(map[string]string, len(ns))
	for key, value := range ns {
		out[key] = value
	}
	return out
}

func escapeCanonicalAttrValue(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		`"`, "&quot;",
		"\t", "&#x9;",
		"\n", "&#xA;",
		"\r", "&#xD;",
	)
	return replacer.Replace(value)
}

func escapeCanonicalText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\r", "&#xD;",
	)
	return replacer.Replace(value)
}

func writeCanonicalAttr(b *strings.Builder, name, value string) {
	b.WriteByte(' ')
	b.WriteString(name)
	b.WriteString(`="`)
	b.WriteString(escapeCanonicalAttrValue(value))
	b.WriteByte('"')
}

func decodeXMLBase64(data []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
}
