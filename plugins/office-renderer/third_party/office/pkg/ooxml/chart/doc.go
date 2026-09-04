// Package chart provides parsing and SVG rendering for OOXML charts.
//
// This package handles chart parts from Excel (XLSX), Word (DOCX), and
// PowerPoint (PPTX) files. Charts are parsed from c:chartSpace XML
// structures and rendered as SVG.
//
// Basic usage:
//
//	import "office/pkg/ooxml/chart"
//
//	// Parse chart from XML
//	c, err := chart.ParseChart(xmlData)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Render as SVG
//	svg := c.SVG(600, 400)
//
// Supported chart types:
//   - Bar (horizontal)
//   - Column (vertical)
//   - Line
//   - Pie
//   - Doughnut
//   - Area
//   - Scatter
//   - Bubble
//   - Stock / OHLC / HLC
//   - Radar
//   - Surface
//
// The package extracts chart data from the cached values in the OOXML
// (numCache/strCache elements), which ensures charts render correctly
// even when external data links are not available.
package chart
