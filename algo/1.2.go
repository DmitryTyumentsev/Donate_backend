package algo

//func firstChar(s string) (rune, int, bool) {
//	runes := []rune(s)
//	seen := make(map[rune]int)
//	for k, v := range runes {
//		seen[v] = k
//	}
//	for i := 0; i < len(runes); i++ {
//		if target, ok := seen[v]; ok {
//			return v, target, true
//		}
//	}
//
//	return 0, 0, false
//}

func firstCharEasySolve(s string) (rune, int, bool) {
	if len(s) == 0 {
		return 0, 0, false
	}
	runes := []rune(s)
	for k, v := range runes {
		unique := true
		for i, r := range runes {
			if k != i && r == v {
				unique = false
				break
			}
		}
		if unique {
			return v, k, true
		}

		//не понимаю как даже такое решение работает. что понимаю: мы двойным циклом каждую пару k, v сравниваем с каждым элементом слайса и если есть повтор прекращаем внутренний цикл и переходим к следующей итерации. сравнили, перебрали
	}
	return 0, 0, false
}
