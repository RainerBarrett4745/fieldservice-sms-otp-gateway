package main

import (
	"context"
	"errors"
	"sync"
)

type OTPGateway interface {
	SendOTP(context.Context, string, string) error
	VerifyOTP(context.Context, string, string) error
}

type WorkOrderLogin struct {
	WorkOrderID string   `json:"work_order_id"`
	Phone       string   `json:"phone"`
	PhotoURLs   []string `json:"photo_urls"`
	Status      string   `json:"dispatch_status"`
	FollowUp    string   `json:"technician_follow_up"`
}

type DispatchLogin struct {
	mu      sync.Mutex
	sms     OTPGateway
	pending map[string]WorkOrderLogin
}

func NewDispatchLogin(sms OTPGateway) *DispatchLogin {
	return &DispatchLogin{sms: sms, pending: make(map[string]WorkOrderLogin)}
}

func (d *DispatchLogin) Start(ctx context.Context, login WorkOrderLogin, requestID string) error {
	if login.WorkOrderID == "" || login.Phone == "" || requestID == "" {
		return errors.New("work_order_id, phone, and request_id are required")
	}
	if err := d.sms.SendOTP(ctx, login.Phone, requestID); err != nil {
		return err
	}
	login.Status = "code_sent"
	d.mu.Lock()
	d.pending[login.Phone] = login
	d.mu.Unlock()
	return nil
}

func (d *DispatchLogin) Verify(ctx context.Context, phone, code string) (WorkOrderLogin, error) {
	d.mu.Lock()
	login, found := d.pending[phone]
	d.mu.Unlock()
	if !found {
		return WorkOrderLogin{}, errors.New("no pending login for phone")
	}
	if err := d.sms.VerifyOTP(ctx, phone, code); err != nil {
		return WorkOrderLogin{}, err
	}
	login.Status = "technician_authenticated"
	d.mu.Lock()
	delete(d.pending, phone)
	d.mu.Unlock()
	return login, nil
}
