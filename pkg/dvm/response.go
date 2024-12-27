package dvm

// These dummy funcs belong to another library

type RankResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

// type ImpersonatorResponse struct {
// 	Pubkey    string  `json:"pubkey"`
// 	Gpr       float64 `json:"gpr"`
// 	Ppr       float64 `json:"ppr"`
// 	Warning   bool    `json:"warning"`
// 	Candidate bool    `json:"candidate"`
// // }

// const fakeDelay = 1 * time.Second

// func RelevantWhoFollow(source string, target string, distance int, sort string, limit int) []RankResponse {
// 	time.Sleep(fakeDelay)
// 	return []RankResponse{
// 		{Pubkey: "bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62", Gpr: 0.025, Ppr: 0.173},
// 		{Pubkey: "d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea", Gpr: 0.022, Ppr: 0.163},
// 		{Pubkey: "6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2", Gpr: 0.021, Ppr: 0.154},
// 		{Pubkey: "bb17f1e4e516e75e82a5b5e81c0120ffeb24e9e92866962440b9888ae82e42a1", Gpr: 0.02, Ppr: 0.111},
// 		{Pubkey: "5097f9b8bd3ebb3c240a6a0c95bdf24d22a10211181f90ba29c41c31c889ba0a", Gpr: 0.022, Ppr: 0.107},
// 	}
// }

// func RecommendedFollows(source string, distance int, sort string, limit int) []RankResponse {
// 	time.Sleep(fakeDelay)
// 	return []RankResponse{
// 		{Pubkey: "7f5b06fa0a104585938c8eb4b5cd4e2b1bd0c0615960ff21214aee09041fdf62", Gpr: 0.035, Ppr: 0.273},
// 		{Pubkey: "af7c905827bc83e1330bee2c3a6a5b86efd1b46b3e6d1ec2447e0769734785e2", Gpr: 0.031, Ppr: 0.254},
// 		{Pubkey: "95bdf24d22a10211181f90ba29c41c5097f9b8bd3ebb3c240a6a0c31c889ba0a", Gpr: 0.032, Ppr: 0.207},
// 	}
// }

// func SortAuthors(source string, targets []string, sort string) []RankResponse {
// 	time.Sleep(fakeDelay)
// 	return []RankResponse{
// 		{Pubkey: "bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62", Gpr: 0.025, Ppr: 0.173},
// 		{Pubkey: "d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea", Gpr: 0.022, Ppr: 0.163},
// 		{Pubkey: "6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2", Gpr: 0.023, Ppr: 0.154},
// 	}
// }

// func ImpersonatorDetection(source string, target string, sort string) []ImpersonatorResponse {
// 	time.Sleep(fakeDelay)
// 	return []ImpersonatorResponse{
// 		{Pubkey: "2447af7c905827bc83e1330bee26efd1b46b3e6d1ecc3a6a5b8e0769734785e2", Gpr: 0.045, Ppr: 0.043, Candidate: true},
// 		{Pubkey: "cc359ef6b8599c820d05ab982e1105476ab68e4c6728d148f8e6222154e60bea", Gpr: 0.021, Ppr: 0.001, Warning: true},
// 	}
// }

// func DegreesOfSeparation(source string, target string, sort string) int {
// 	time.Sleep(fakeDelay)
// 	return 3
// }
