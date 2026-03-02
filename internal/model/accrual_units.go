package model

import (
	"errors"
	"math"
)

// AccrualScale is the multiplier between major and minor loyalty point units.
// We store amounts in minor units (uint64) across the domain.
const AccrualScale = 100

var (
	// ErrAccrualInvalidMajorAmount возвращается, когда значение в мажорных единицах некорректно: отрицательное, NaN
	// или бесконечность.
	ErrAccrualInvalidMajorAmount = errors.New("invalid accrual major amount")

	// ErrAccrualMajorOverflow возвращается, когда после масштабирования и округления значение не помещается в uint64
	// (минорные единицы).
	ErrAccrualMajorOverflow = errors.New("accrual major amount overflows minor units")
)

// AccrualMajorToMinor переводит вещественное представление мажорных единиц баллов лояльности (float64) в целочисленное
// представление в минорных единицах баллов лояльности (uint64) с округлением до ближайшей минорной единицы.
func AccrualMajorToMinor(u float64) (uint64, error) {
	if math.IsNaN(u) || math.IsInf(u, 0) || u < 0 {
		return 0, ErrAccrualInvalidMajorAmount
	}
	scaled := math.Round(u * AccrualScale)
	if scaled < 0 || scaled > float64(^uint64(0)) {
		return 0, ErrAccrualMajorOverflow
	}
	return uint64(scaled), nil
}
