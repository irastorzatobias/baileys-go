package send

type PresenceRequest struct {
	Type        string `json:"type" form:"type"`
	IsForwarded bool   `json:"is_forwarded" form:"is_forwarded"`
	CompanyNid  string `json:"company_nid,omitempty" form:"company_nid"`
}
