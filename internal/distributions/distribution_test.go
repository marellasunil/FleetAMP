package distributions

import (
	"testing"
	"time"
)

func TestNewDistributionCreatesStableImmutableEvidence(t *testing.T) {
	input := ComponentSet{Receivers: []string{"prometheus", "otlp", "otlp"}, Processors: []string{"batch"}, Exporters: []string{"otlphttp"}}
	one, err := New("Platform Collector", "0.140.0", "v0.140.0", "oci://registry/collector:0.140.0", "", input, []string{"exporters"}, []string{"receivers"}, "admin", time.Unix(1,0))
	if err != nil { t.Fatal(err) }
	two, err := New("Platform Collector", "0.140.0", "v0.140.0", "oci://registry/collector:0.140.0", "", input, []string{"exporters"}, []string{"receivers"}, "other-admin", time.Unix(2,0))
	if err != nil { t.Fatal(err) }
	if one.SpecHash == "" || one.SpecHash != two.SpecHash || one.ID != two.ID { t.Fatalf("stable evidence mismatch: %#v %#v",one,two) }
	if len(one.Components.Receivers) != 2 { t.Fatalf("receivers=%v",one.Components.Receivers) }
}

func TestDistributionRejectsMutableSourceAndOverlappingPolicy(t *testing.T) {
	components := ComponentSet{Receivers: []string{"otlp"}}
	if _,err:=New("Collector","0.140.0","latest","","",components,nil,nil,"admin",time.Now());err==nil{t.Fatal("expected mutable source rejection")}
	if _,err:=New("Collector","0.140.0","v0.140.0","","",components,[]string{"exporters"},[]string{"exporters"},"admin",time.Now());err==nil{t.Fatal("expected overlapping section rejection")}
}
