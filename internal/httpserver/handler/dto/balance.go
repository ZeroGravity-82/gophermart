package dto

// BalanceResponse — ответ ручки GET /api/user/balance.
// Баллы лояльности передаются в мажорных единицах (float64) с точностью до 2 знаков.
type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
