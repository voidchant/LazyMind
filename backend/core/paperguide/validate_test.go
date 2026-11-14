package paperguide

import "testing"

func TestValidateEvidenceRejectsForeignAndTamperedLocators(t *testing.T) {
	d, identity := fixture(t)
	s, err := BuildSourceSnapshot(d, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(s, identity, s.IRDigest); err != nil {
		t.Fatal(err)
	}
	source := s.Evidence[1]
	if err := ValidateEvidence(s, source); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*GuideEvidence){
		func(e *GuideEvidence) { e.ID = "fabricated" },
		func(e *GuideEvidence) { e.Locator.Page = 300 },
		func(e *GuideEvidence) { e.Locator.End = 1000 },
		func(e *GuideEvidence) { e.Text = "fake excerpt" },
	} {
		copy := source
		edit(&copy)
		if err := ValidateEvidence(s, copy); err == nil {
			t.Fatalf("accepted %+v", copy)
		}
	}
}

func TestPlanSectionAndReviewValidateScientificProseAndScope(t *testing.T) {
	d, identity := fixture(t)
	s, err := BuildSourceSnapshot(d, identity)
	if err != nil {
		t.Fatal(err)
	}
	a := ArgumentExperiment
	claim := GuideClaim{ID: "c1", Statement: "Every sample with x < y changed by −2 g.", Attribution: AttributionPaper,
		ArgumentType: &a, EvidenceIDs: []string{s.Evidence[0].ID}, SupportStatus: SupportSupported}
	p := GuidePlan{Profiles: GuideProfiles{Primary: &a, Secondary: []ArgumentType{ArgumentObservational}},
		Questions: []string{"What did the authors weigh?"}, Claims: []GuideClaim{claim}}
	for _, role := range Roles() {
		p.Sections = append(p.Sections, GuideSectionPlan{Role: role, Title: string(role), Focus: "Explain x < y.", ClaimIDs: []string{"c1"}})
	}
	if err := ValidatePlan(s, p); err != nil {
		t.Fatal(err)
	}
	block := GuideBlock{ID: "b1", Kind: "paragraph", Text: claim.Statement, Attribution: AttributionPaper, ClaimIDs: []string{claim.ID}}
	section := GuideSection{Role: RoleBackground, Title: "background", Summary: "Samples at −2 °C.", Blocks: []GuideBlock{block}, ClaimIDs: []string{"c1"}}
	if err := ValidateSection(s, p, RoleBackground, section); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{`<script>run()</script>`, `![](https://remote.example)`, `![[external-attachment]]`} {
		section.Blocks[0].Text = text
		if err := ValidateSection(s, p, RoleBackground, section); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	review := GuideReview{Findings: []GuideFinding{{TargetID: "b1", TargetType: "block", IssueType: "missing_condition", Suggestion: "Restore the range x < y."}}}
	if err := ValidateReview(p, []GuideSection{section}, review); err != nil {
		t.Fatal(err)
	}
	review.Findings[0].TargetID = "foreign"
	if err := ValidateReview(p, []GuideSection{section}, review); err == nil {
		t.Fatal("accepted a foreign block")
	}
	p.Claims[0].EvidenceIDs[0] = "ev_foreign"
	if err := ValidatePlan(s, p); err == nil {
		t.Fatal("accepted a foreign claim source")
	}
}
