package edit

import (
	"testing"

	ooxmlchart "office/pkg/ooxml/chart"
)

func TestRunSetChart(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()

	c := &ooxmlchart.Chart{ChartType: ooxmlchart.ChartBar, Title: "Sales"}
	r.SetChart(c)

	if r.ir().Kind != "chart" {
		t.Fatalf("kind = %q, want chart", r.ir().Kind)
	}
	if r.ir().Chart == nil {
		t.Fatal("expected chart")
	}
	if r.Chart() == nil {
		t.Fatal("expected Chart editor")
	}
	if r.Chart().Title() != "Sales" {
		t.Fatalf("title = %q", r.Chart().Title())
	}
}

func TestRunAddChart(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	c := r.AddChart()
	if c == nil {
		t.Fatal("expected chart")
	}
	c.SetKind("column")
	c.SetTitle("Revenue")
	if r.Chart().Title() != "Revenue" {
		t.Fatalf("title = %q", r.Chart().Title())
	}
}

func TestDocumentCharts(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	r1 := p1.AddRun()
	r1.SetChart(&ooxmlchart.Chart{ChartType: ooxmlchart.ChartPie})

	p2 := d.AddParagraph()
	r2 := p2.AddRun()
	r2.SetChart(&ooxmlchart.Chart{ChartType: ooxmlchart.ChartLine})

	charts := d.Charts()
	if len(charts) != 2 {
		t.Fatalf("charts = %d, want 2", len(charts))
	}
}

func TestChartSeries(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	c := r.AddChart()
	c.SetKind("bar")

	s1 := c.AddSeries("Q1", []float64{10, 20, 30})
	s2 := c.AddSeries("Q2", []float64{15, 25, 35})

	if c.SeriesCount() != 2 {
		t.Fatalf("count = %d, want 2", c.SeriesCount())
	}

	s1.SetName("Quarter 1")
	if c.Series(0).Name() != "Quarter 1" {
		t.Fatalf("name = %q", c.Series(0).Name())
	}

	s2.SetValues([]float64{100, 200, 300})
	vals := c.Series(1).Values()
	if len(vals) != 3 || vals[0] != 100 {
		t.Fatalf("values = %v", vals)
	}

	c.RemoveSeries(0)
	if c.SeriesCount() != 1 {
		t.Fatalf("count after remove = %d, want 1", c.SeriesCount())
	}
	if c.Series(0).Name() != "Q2" {
		t.Fatalf("name after remove = %q", c.Series(0).Name())
	}
}

func TestChartCategories(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	c := r.AddChart()
	c.SetCategories([]string{"A", "B", "C"})
	cats := c.Categories()
	if len(cats) != 3 || cats[1] != "B" {
		t.Fatalf("categories = %v", cats)
	}
}

func TestRunRemoveChart(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.AddChart()
	r.RemoveChart()
	if r.ir().Chart != nil {
		t.Fatal("expected nil chart")
	}
	if r.ir().Kind != "run" {
		t.Fatalf("kind = %q, want run", r.ir().Kind)
	}
}