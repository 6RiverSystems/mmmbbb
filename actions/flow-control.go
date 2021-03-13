package actions

type FlowControl struct {
	MaxMessages int `json:"maxMessages" form:"maxMessages,default=1" binding:"gte=1"`
	MaxBytes    int `json:"maxBytes" form:"maxBytes,default=1" binding:"gte=1"`
}
