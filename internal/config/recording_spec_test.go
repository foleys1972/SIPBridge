package config

import "testing"

func TestValidateRecordingSpec_multiTrunk(t *testing.T) {
	r := &RecordingSpec{
		GlobalEnabled: true,
		SIPREC: &SIPRECIntegrationSpec{
			Enabled: true,
			Trunks: []RecordingTrunkEntry{
				{ID: "us", RecorderSIPURI: "sip:rs-us@rec.example.com"},
				{ID: "eu", RecorderSIPURI: "sip:rs-eu@rec.example.com"},
			},
			DefaultTrunkID: "us",
			RegionToTrunk: map[string]string{
				"US": "us",
				"EMEA": "eu",
			},
		},
	}
	if err := ValidateRecordingSpec(r); err != nil {
		t.Fatal(err)
	}
}

func TestSelectRecordingTrunkForRegion(t *testing.T) {
	s := &SIPRECIntegrationSpec{
		Enabled: true,
		Trunks: []RecordingTrunkEntry{
			{ID: "us", RecorderSIPURI: "sip:rs-us@x"},
			{ID: "eu", RecorderSIPURI: "sip:rs-eu@x"},
		},
		DefaultTrunkID: "eu",
		RegionToTrunk: map[string]string{
			"US": "us",
		},
	}
	got, ok := SelectRecordingTrunkForRegion(s, "US")
	if !ok || got.ID != "us" || got.RecorderSIPURI != "sip:rs-us@x" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	got, ok = SelectRecordingTrunkForRegion(s, "us-west")
	if !ok || got.ID != "eu" {
		t.Fatalf("expected default eu, got %+v ok=%v", got, ok)
	}
}

func TestSelectRecordingTrunkForRegion_legacy(t *testing.T) {
	s := &SIPRECIntegrationSpec{
		Enabled:        true,
		RecorderSIPURI: "sip:legacy@x",
	}
	got, ok := SelectRecordingTrunkForRegion(s, "")
	if !ok || got.RecorderSIPURI != "sip:legacy@x" {
		t.Fatalf("got %+v", got)
	}
}
