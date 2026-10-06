package lifecycle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type ExecutorKind string

const (
	ExecutorGitOps     ExecutorKind = "gitops"
	ExecutorKubernetes ExecutorKind = "kubernetes-api"
	ExecutorSystemd    ExecutorKind = "systemd"
	ExecutorContainer  ExecutorKind = "container-runtime"
	ExecutorManual     ExecutorKind = "manual-package"
)

// Executor is the vendor-neutral boundary future delivery adapters must implement.
// No implementation is registered by the preparation milestone.
type Executor interface {
	Kind() ExecutorKind
	Execute(context.Context, *ExecutionPlan) (*ExecutionReceipt, error)
}

type ExecutionReceipt struct {
	PlanID       string    `json:"plan_id"`
	PlanHash     string    `json:"plan_hash"`
	ExecutorKind ExecutorKind `json:"executor_kind"`
	ExternalID   string    `json:"external_id"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type ExecutionPlan struct {
	ID                 string           `json:"id"`
	ApprovalID         string           `json:"approval_id"`
	RequestID          string           `json:"request_id"`
	RequestSpecHash    string           `json:"request_spec_hash"`
	ValidationID       string           `json:"validation_id"`
	ValidationHash     string           `json:"validation_hash"`
	ExecutorKind       ExecutorKind     `json:"executor_kind"`
	Operation          Operation        `json:"operation"`
	ComponentType      string           `json:"component_type"`
	DeploymentMethod   string           `json:"deployment_method"`
	Targets            []TargetSnapshot `json:"targets"`
	PlanHash           string           `json:"plan_hash"`
	PreparedBy         string           `json:"prepared_by"`
	CreatedAt          time.Time        `json:"created_at"`
}

func PrepareExecution(approval *Approval, request *Request, validation *Validation, preparedBy string) (*ExecutionPlan, error) {
	if approval == nil || request == nil || validation == nil { return nil, errors.New("approval, proposal, and validation are required") }
	if approval.Status != ApprovalApproved { return nil, errors.New("lifecycle request is not approved") }
	if approval.RequestID != request.ID || approval.RequestSpecHash != request.SpecHash || approval.ValidationID != validation.ID || approval.ValidationHash != validation.ResultHash {
		return nil, errors.New("approved evidence hashes do not match stored immutable artifacts")
	}
	if validation.RequestID != request.ID || validation.RequestSpecHash != request.SpecHash || validation.Status != ValidationCompatible || len(validation.Targets) != approval.TargetCount {
		return nil, errors.New("validation evidence is no longer eligible for execution preparation")
	}
	preparedBy = strings.TrimSpace(preparedBy); if preparedBy == "" { return nil, errors.New("preparer is required") }
	kind := ExecutorKind(request.Spec.DeploymentMethod)
	switch kind { case ExecutorGitOps,ExecutorKubernetes,ExecutorSystemd,ExecutorContainer,ExecutorManual: default: return nil, errors.New("no executor contract exists for the deployment method") }
	raw:=make([]byte,16);if _,err:=rand.Read(raw);err!=nil{return nil,err}
	plan:=&ExecutionPlan{ID:hex.EncodeToString(raw),ApprovalID:approval.ID,RequestID:request.ID,RequestSpecHash:request.SpecHash,
		ValidationID:validation.ID,ValidationHash:validation.ResultHash,ExecutorKind:kind,Operation:request.Spec.Operation,
		ComponentType:string(request.Spec.ComponentType),DeploymentMethod:request.Spec.DeploymentMethod,Targets:append([]TargetSnapshot(nil),validation.Targets...),PreparedBy:preparedBy,CreatedAt:time.Now().UTC()}
	for i:=range plan.Targets{plan.Targets[i].Labels=cloneStrings(validation.Targets[i].Labels)}
	hashInput:=struct{ApprovalID string `json:"approval_id"`;RequestID string `json:"request_id"`;RequestSpecHash string `json:"request_spec_hash"`;ValidationID string `json:"validation_id"`;ValidationHash string `json:"validation_hash"`;ExecutorKind ExecutorKind `json:"executor_kind"`;Operation Operation `json:"operation"`;ComponentType string `json:"component_type"`;DeploymentMethod string `json:"deployment_method"`;Targets []TargetSnapshot `json:"targets"`}{plan.ApprovalID,plan.RequestID,plan.RequestSpecHash,plan.ValidationID,plan.ValidationHash,plan.ExecutorKind,plan.Operation,plan.ComponentType,plan.DeploymentMethod,plan.Targets}
	encoded,err:=json.Marshal(hashInput);if err!=nil{return nil,err};digest:=sha256.Sum256(encoded);plan.PlanHash=hex.EncodeToString(digest[:]);return plan,nil
}

func CloneExecutionPlan(value *ExecutionPlan)*ExecutionPlan{if value==nil{return nil};copy:=*value;copy.Targets=append([]TargetSnapshot(nil),value.Targets...);for i:=range copy.Targets{copy.Targets[i].Labels=cloneStrings(value.Targets[i].Labels)};return &copy}
