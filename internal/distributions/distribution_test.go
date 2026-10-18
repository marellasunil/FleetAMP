package distributions

import (
	"testing"
	"time"
)

func TestNewDistributionCreatesStableImmutableEvidence(t *testing.T) {
	baseline := "receivers:\\n  otlp: {}\\nexporters:\\n  otlphttp: {}\\nservice:\\n  pipelines:\\n    traces:\\n      receivers: [otlp]\\n      exporters: [otlphttp]"
	input := ComponentSet{Receivers: []string{"prometheus", "otlp", "otlp"}, Processors: []string{"batch"}, Exporters: []string{"otlphttp"}}
	one, err := New("Platform Collector", "0.140.0", "v0.140.0", "oci://registry/collector:0.140.0", "", input, baseline, []string{"exporters"}, []string{"receivers"}, "admin", time.Unix(1,0))
	if err != nil { t.Fatal(err) }
	two, err := New("Platform Collector", "0.140.0", "v0.140.0", "oci://registry/collector:0.140.0", "", input, baseline, []string{"exporters"}, []string{"receivers"}, "other-admin", time.Unix(2,0))
	if err != nil { t.Fatal(err) }
	if one.SpecHash == "" || one.SpecHash != two.SpecHash || one.ID != two.ID { t.Fatalf("stable evidence mismatch: %#v %#v",one,two) }
	if len(one.Components.Receivers) != 2 { t.Fatalf("receivers=%v",one.Components.Receivers) }
	changed,err:=New("Platform Collector","0.140.0","v0.140.0","oci://registry/collector:0.140.0","",input,baseline+"\\n# changed",[]string{"exporters"},[]string{"receivers"},"admin",time.Unix(1,0));if err!=nil{t.Fatal(err)};if changed.SpecHash==one.SpecHash{t.Fatal("baseline change must change immutable specification hash")}
}

func TestDistributionRejectsMutableSourceAndOverlappingPolicy(t *testing.T) {
	components := ComponentSet{Receivers: []string{"otlp"}}
	baseline := "receivers:\\n  otlp: {}"
	if _,err:=New("Collector","0.140.0","latest","","",components,baseline,nil,nil,"admin",time.Now());err==nil{t.Fatal("expected mutable source rejection")}
	if _,err:=New("Collector","0.140.0","v0.140.0","","",components,baseline,[]string{"exporters"},[]string{"exporters"},"admin",time.Now());err==nil{t.Fatal("expected overlapping section rejection")}
}
