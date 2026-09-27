package main

import "fmt"

func twoSum(nums []int, target int) []int {
	var res []int
	for i := 0; i < len(nums); i++ {
		for j := i + 1; j < len(nums); j++ {
			if i != j && (nums[i]+nums[j]) == target {
				res = append(res, i, j)
				return res
			}
		}
	}
	return res
}

func main() {
	fmt.Println(twoSum([]int{2, 7, 11, 15}, 9))
}
