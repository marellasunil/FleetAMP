package lifecycle

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestPrepareExecutionVerifiesApprovedHashes(t *testing.T){
	request,err:=NewRequest(Spec{Operation:Upgrade,ComponentType:runtimes.OTelCollectorKubernetes,GroupID:"payments",DeploymentMethod:"gitops",CurrentVersion:"0.148.0",DesiredVersion:"0.149.0",Reason:"security update"},"operator");if err!=nil{t.Fatal(err)}
	validation:=&Validation{ID:"validation-1",RequestID:request.ID,RequestSpecHash:request.SpecHash,ResultHash:"validation-hash",GroupID:"payments",GroupName:"Payments",Status:ValidationCompatible,Targets:[]TargetSnapshot{{InstanceUID:"collector-a",Labels:map[string]string{"environment":"production"}}}}
	approval,err:=NewApproval(request,validation,"operator","admin","review");if err!=nil{t.Fatal(err)};approval.Status=ApprovalApproved
	plan,err:=PrepareExecution(approval,request,validation,"admin");if err!=nil{t.Fatal(err)}
	if plan.ExecutorKind!=ExecutorGitOps||plan.PlanHash==""||len(plan.Targets)!=1{t.Fatalf("unexpected plan: %#v",plan)}
	validation.Targets[0].Labels["environment"]="changed";if plan.Targets[0].Labels["environment"]!="production"{t.Fatal("plan target snapshot changed")}
	approval.ValidationHash="tampered";if _,err:=PrepareExecution(approval,request,validation,"admin");err==nil{t.Fatal("tampered validation hash accepted")}
}

func TestPrepareExecutionRejectsUnapprovedRequest(t *testing.T){
	request,_:=NewRequest(Spec{Operation:Restart,ComponentType:runtimes.OTelCollector,GroupID:"payments",DeploymentMethod:"systemd",CurrentVersion:"0.149.0",Reason:"restart"},"operator")
	validation:=&Validation{ID:"v",RequestID:request.ID,RequestSpecHash:request.SpecHash,ResultHash:"hash",Status:ValidationCompatible,Targets:[]TargetSnapshot{{InstanceUID:"a"}}}
	approval,_:=NewApproval(request,validation,"operator","admin","review")
	if _,err:=PrepareExecution(approval,request,validation,"admin");err==nil{t.Fatal("pending approval accepted")}
}
