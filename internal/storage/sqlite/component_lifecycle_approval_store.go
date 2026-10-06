package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentLifecycleApprovalStore struct{ db *sql.DB }
func (s *ComponentLifecycleApprovalStore) Create(ctx context.Context,value *lifecycle.Approval) error { _,err:=s.db.ExecContext(ctx,`INSERT INTO component_lifecycle_approvals (id,request_id,request_spec_hash,validation_id,validation_hash,group_id,group_name,operation,component_type,target_count,requested_by,assigned_reviewer,submission_comment,status,reviewed_by,review_comment,reviewed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,value.ID,value.RequestID,value.RequestSpecHash,value.ValidationID,value.ValidationHash,value.GroupID,value.GroupName,value.Operation,value.ComponentType,value.TargetCount,value.RequestedBy,value.AssignedReviewer,value.SubmissionComment,value.Status,value.ReviewedBy,value.ReviewComment,formatTimePtr(value.ReviewedAt),formatTime(value.CreatedAt)); if err!=nil{return fmt.Errorf("create component lifecycle approval: %w",err)}; return nil }
func (s *ComponentLifecycleApprovalStore) Get(ctx context.Context,id string)(*lifecycle.Approval,error){value,err:=scanLifecycleApproval(s.db.QueryRowContext(ctx,lifecycleApprovalSelect+` WHERE id=?`,id));if errors.Is(err,sql.ErrNoRows){return nil,storage.ErrComponentLifecycleApprovalNotFound};return value,err}
func (s *ComponentLifecycleApprovalStore) List(ctx context.Context,limit int)([]*lifecycle.Approval,error){if limit<=0||limit>500{limit=100};rows,err:=s.db.QueryContext(ctx,lifecycleApprovalSelect+` ORDER BY created_at DESC LIMIT ?`,limit);if err!=nil{return nil,err};defer rows.Close();result:=[]*lifecycle.Approval{};for rows.Next(){value,err:=scanLifecycleApproval(rows);if err!=nil{return nil,err};result=append(result,value)};return result,rows.Err()}
func (s *ComponentLifecycleApprovalStore) Review(ctx context.Context,id string,from,to lifecycle.ApprovalStatus,reviewer,comment string)error{now:=time.Now().UTC();result,err:=s.db.ExecContext(ctx,`UPDATE component_lifecycle_approvals SET status=?,reviewed_by=?,review_comment=?,reviewed_at=? WHERE id=? AND status=?`,to,reviewer,comment,formatTime(now),id,from);if err!=nil{return err};count,err:=result.RowsAffected();if err!=nil{return err};if count!=1{return storage.ErrComponentLifecycleApprovalConflict};return nil}
const lifecycleApprovalSelect=`SELECT id,request_id,request_spec_hash,validation_id,validation_hash,group_id,group_name,operation,component_type,target_count,requested_by,assigned_reviewer,submission_comment,status,reviewed_by,review_comment,reviewed_at,created_at FROM component_lifecycle_approvals`
type lifecycleApprovalScanner interface{Scan(...any)error}
func scanLifecycleApproval(scanner lifecycleApprovalScanner)(*lifecycle.Approval,error){value:=&lifecycle.Approval{};var operation,status,createdAt string;var reviewedAt sql.NullString;if err:=scanner.Scan(&value.ID,&value.RequestID,&value.RequestSpecHash,&value.ValidationID,&value.ValidationHash,&value.GroupID,&value.GroupName,&operation,&value.ComponentType,&value.TargetCount,&value.RequestedBy,&value.AssignedReviewer,&value.SubmissionComment,&status,&value.ReviewedBy,&value.ReviewComment,&reviewedAt,&createdAt);err!=nil{return nil,err};value.Operation=lifecycle.Operation(operation);value.Status=lifecycle.ApprovalStatus(status);value.CreatedAt,_=parseTime(createdAt);if reviewedAt.Valid{parsed,_:=parseTime(reviewedAt.String);value.ReviewedAt=&parsed};return value,nil}
var _ storage.ComponentLifecycleApprovalStore=(*ComponentLifecycleApprovalStore)(nil)
