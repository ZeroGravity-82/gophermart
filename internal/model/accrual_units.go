package model

import "math"

// AccrualScale is the multiplier between major and minor loyalty point units.
// We store amounts in minor units (uint64) across the domain.
const AccrualScale = 100

// AccrualMajorToMinor переводит вещественное представление мажорных единиц баллов лояльности (float64) в целочисленное
// представление в минорных единицах баллов лояльности (uint64) с округлением до ближайшей минорной единицы.
func AccrualMajorToMinor(u float64) (uint64, bool) {
	if math.IsNaN(u) || math.IsInf(u, 0) || u < 0 {
		return 0, false
	}
	scaled := math.Round(u * AccrualScale)
	if scaled < 0 || scaled > float64(^uint64(0)) {
		return 0, false
	}
	return uint64(scaled), true
}
