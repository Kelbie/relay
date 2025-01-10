package tests

// pubkeys for testing purposes
const (
	fran  string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip   string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
)

// func TestRelevantWhoFollow(t *testing.T) {

// 	req := &nostr.Event{
// 		PubKey: pip,
// 		Kind:   dvm.KindRelevantWhoFollow,
// 		Tags: nostr.Tags{
// 			{"param", "source", odell},
// 			{"param", "target", fran},
// 		},
// 	}

// 	_ = req
// 	sk := nostr.GeneratePrivateKey()
// 	pk, err := nostr.GetPublicKey(sk)
// 	if err != nil {
// 		t.Fatalf("test failed: %v", err)
// 	}

// 	t.Errorf("sk: %v", sk)
// 	t.Errorf("pk: %v", pk)
// }
