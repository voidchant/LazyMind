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
	claim := GuideClaim{ID: "c1", Statement: "Every sample with $a<x>b$ changed by −2 g.", Attribution: AttributionPaper,
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
	for _, text := range []string{`<script>run()</script>`, `$a<script>alert(1)</script>b$`, `$<img src=x>$`, `![](https://remote.example)`, `![[external-attachment]]`} {
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

func TestSectionRequiresPlannedClaimsAndEvidenceClosure(t *testing.T) {
	d, id := fixture(t)
	s, err := BuildSourceSnapshot(d, id)
	if err != nil {
		t.Fatal(err)
	}
	statement := "Each sample was weighed."
	plan := GuidePlan{Sections: []GuideSectionPlan{{Role: RoleBackground, Title: "Background", ClaimIDs: []string{"required"}}},
		Claims: []GuideClaim{
			{ID: "required", EvidenceIDs: []string{s.Evidence[0].ID}},
			{ID: "another", EvidenceIDs: []string{s.Evidence[1].ID}},
		}}
	out := GuideSection{Role: RoleBackground, Title: "Background", Summary: statement, ClaimIDs: []string{"required"},
		Blocks: []GuideBlock{{ID: "b1", Kind: "paragraph", Text: statement, Attribution: AttributionPaper, ClaimIDs: []string{"required"}}}}
	packet, err := RestrictEvidence(s, []string{s.Evidence[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSection(packet, plan, RoleBackground, out); err != nil {
		t.Fatal(err)
	}
	other, err := RestrictEvidence(s, []string{s.Evidence[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSection(other, plan, RoleBackground, out); err == nil {
		t.Error("accepted frozen claim outside packet")
	}
	out.ClaimIDs, out.Blocks[0].ClaimIDs = []string{"another"}, []string{"another"}
	if err := ValidateSection(s, plan, RoleBackground, out); err == nil {
		t.Error("omitted required claim")
	}
	out.ClaimIDs, out.Blocks[0].ClaimIDs = []string{"required", "another"}, []string{"required", "another"}
	if err := ValidateSection(s, plan, RoleBackground, out); err != nil {
		t.Fatal(err)
	}
}

func TestReviewRequiresUnambiguousTargetsEvenWithoutFindings(t *testing.T) {
	plan := GuidePlan{Claims: []GuideClaim{{ID: "frozen"}}}
	sections := []GuideSection{
		{Role: RoleBackground, Blocks: []GuideBlock{{ID: "b1", ClaimIDs: []string{"frozen"}}}, Claims: []GuideClaim{{ID: "n1"}}},
		{Role: RoleQuestion, Blocks: []GuideBlock{{ID: "b2", ClaimIDs: []string{"frozen"}}}, Claims: []GuideClaim{{ID: "n2"}}},
		{Role: RoleContribution}, {Role: RoleJustification},
	}
	finding := GuideFinding{TargetType: "block", TargetID: "b1", IssueType: "coverage", Suggestion: "Explain what went unreported."}
	for _, subset := range [][]GuideSection{sections, sections[:1]} {
		if err := ValidateReview(plan, subset, GuideReview{Findings: []GuideFinding{finding}}); err != nil {
			t.Fatal(err)
		}
	}
	sections[1].Blocks[0].ID = "b1"
	if err := ValidateReview(plan, sections, GuideReview{}); err == nil {
		t.Error("ambiguous block")
	}
	sections[1].Blocks[0].ID = "b2"
	for _, id := range []string{"n1", "frozen"} {
		sections[1].Claims[0].ID = id
		if err := ValidateReview(plan, sections, GuideReview{}); err == nil {
			t.Errorf("ambiguous claim %s", id)
		}
	}
}
