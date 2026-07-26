package clientapi

import "time"

const (
	SubscriptionStatusTrialing  = "trialing"
	SubscriptionStatusActive    = "active"
	SubscriptionStatusPastDue   = "past_due"
	SubscriptionStatusGrace     = "grace"
	SubscriptionStatusReadOnly  = "read_only"
	SubscriptionStatusSuspended = "suspended"
	SubscriptionStatusCanceled  = "canceled"
	PlanCommunity               = "community"
	PlanTeam                    = "team_self_hosted"
	PlanBusiness                = "business"
	PlanEnterprise              = "enterprise_on_prem"
	PlanMSP                     = "msp_integrator"
)

type Account struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Status         string    `json:"status"`
	BillingCountry string    `json:"billing_country"`
	Currency       string    `json:"currency"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Plan struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	AccountType     string          `json:"account_type"`
	Description     string          `json:"description"`
	Currency        string          `json:"currency"`
	MonthlyPrice    int64           `json:"monthly_price"`
	YearlyPrice     int64           `json:"yearly_price"`
	Public          bool            `json:"public"`
	BillingProvider string          `json:"billing_provider"`
	Features        map[string]bool `json:"features"`
	Limits          map[string]int  `json:"limits"`
	Periods         []string        `json:"periods"`
}

type EntitlementSet struct {
	AccountID    string          `json:"account_id"`
	PlanID       string          `json:"plan_id"`
	PlanStatus   string          `json:"plan_status"`
	Features     map[string]bool `json:"features"`
	Limits       map[string]int  `json:"limits"`
	ComputedAt   time.Time       `json:"computed_at"`
	UpgradeURL   string          `json:"upgrade_url"`
	LicenseID    string          `json:"license_id,omitempty"`
	LicenseUntil *time.Time      `json:"license_until,omitempty"`
}

type UsageSnapshot struct {
	AccountID        string         `json:"account_id"`
	Users            int            `json:"users"`
	Nodes            int            `json:"nodes"`
	Networks         int            `json:"networks"`
	JoinTokensActive int            `json:"join_tokens_active"`
	AuditEvents      int            `json:"audit_events"`
	Limits           map[string]int `json:"limits"`
	ComputedAt       time.Time      `json:"computed_at"`
}

type Subscription struct {
	ID                  string     `json:"id"`
	AccountID           string     `json:"account_id"`
	PlanID              string     `json:"plan_id"`
	Status              string     `json:"status"`
	BillingPeriod       string     `json:"billing_period"`
	Provider            string     `json:"provider"`
	ProviderCustomerID  string     `json:"provider_customer_id,omitempty"`
	ProviderSubID       string     `json:"provider_subscription_id,omitempty"`
	TrialStartsAt       *time.Time `json:"trial_starts_at,omitempty"`
	TrialEndsAt         *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodStarts *time.Time `json:"current_period_starts_at,omitempty"`
	CurrentPeriodEnds   *time.Time `json:"current_period_ends_at,omitempty"`
	CancelAtPeriodEnd   bool       `json:"cancel_at_period_end"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type CheckoutSession struct {
	ID                 string     `json:"id"`
	AccountID          string     `json:"account_id"`
	SubscriptionID     string     `json:"subscription_id,omitempty"`
	PlanID             string     `json:"plan_id"`
	BillingPeriod      string     `json:"billing_period"`
	Provider           string     `json:"provider"`
	Status             string     `json:"status"`
	Amount             int64      `json:"amount"`
	Currency           string     `json:"currency"`
	InvoiceID          string     `json:"invoice_id"`
	ProviderPaymentID  string     `json:"provider_payment_id,omitempty"`
	ProviderStatus     string     `json:"provider_status,omitempty"`
	ConfirmationURL    string     `json:"confirmation_url,omitempty"`
	IdempotencyKey     string     `json:"-"`
	IdempotencyKeyHash string     `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
}

type Invoice struct {
	ID             string        `json:"id"`
	AccountID      string        `json:"account_id"`
	SubscriptionID string        `json:"subscription_id,omitempty"`
	CheckoutID     string        `json:"checkout_id,omitempty"`
	Number         string        `json:"number"`
	Status         string        `json:"status"`
	Provider       string        `json:"provider"`
	Amount         int64         `json:"amount"`
	Currency       string        `json:"currency"`
	DueAt          time.Time     `json:"due_at"`
	PaidAt         *time.Time    `json:"paid_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	Items          []InvoiceItem `json:"items,omitempty"`
}

type InvoiceItem struct {
	ID          string `json:"id"`
	InvoiceID   string `json:"invoice_id"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitAmount  int64  `json:"unit_amount"`
	Amount      int64  `json:"amount"`
}

type BillingCheckoutRequest struct {
	PlanID        string `json:"plan_id"`
	BillingPeriod string `json:"billing_period"`
	SuccessURL    string `json:"success_url,omitempty"`
	FailureURL    string `json:"failure_url,omitempty"`
}
