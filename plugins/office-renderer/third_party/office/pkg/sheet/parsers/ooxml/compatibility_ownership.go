package ooxml

// Package metadata is a first-class parser input, not a fallback parser.
// Worksheet content is SAX-owned and relationship-heavy worksheet objects are
// loaded through parts.LoadRelationshipObjects. No compatibility ownership
// inventory remains in production.
