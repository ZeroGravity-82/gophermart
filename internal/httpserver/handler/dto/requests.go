package dto

// WithdrawRequest — тело запроса для POST /api/user/balance/withdraw.
// Sum передается в мажорных единицах баллов лояльности (float64) с 2 знаками после запятой.
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
