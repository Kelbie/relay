package main

// func RelevantWhoFollowHandler(request *nostr.Event, response chan nostr.Event) {
// 	args, errs := getArguments(request, true)
// 	if errs != nil {
// 		response <- createErrorEvent(errs, request)
// 		return
// 	}
// 	// fmt.Printf("checking %s for", args.Source)
// 	// pp, err := PersonalizedPagerank(
// 	// 	context.Background(),
// 	// 	cl,
// 	// 	args.Source,
// 	// 	100,
// 	// )
// 	// if err != nil {
// 	// 	fmt.Println(err)
// 	// 	response <- createErrorEvent([]error{err}, request)
// 	// 	return
// 	// }
// 	// result := pp
// 	result := RelevantWhoFollow(args.Source, args.Targets[0], args.Distance, args.Sort, args.Limit)
// 	response <- createEvent(result, request)
// }

// func RecommendedFollowsHandler(request *nostr.Event, response chan nostr.Event) {
// 	args, errs := getArguments(request, false) // No targets needed
// 	if errs != nil {
// 		response <- createErrorEvent(errs, request)
// 		return
// 	}
// 	result := RecommendedFollows(args.Source, args.Distance, args.Sort, args.Limit)
// 	response <- createEvent(result, request)
// }

// func SortAuthorsHandler(request *nostr.Event, response chan nostr.Event) {
// 	args, errs := getArguments(request, true)
// 	if errs != nil {
// 		response <- createErrorEvent(errs, request)
// 		return
// 	}
// 	result := SortAuthors(args.Source, args.Targets, args.Sort)
// 	response <- createEvent(result, request)
// }

// func ImpersonatorDetectionHandler(request *nostr.Event, response chan nostr.Event) {
// 	args, errs := getArguments(request, true)
// 	if errs != nil {
// 		response <- createErrorEvent(errs, request)
// 		return
// 	}
// 	result := ImpersonatorDetection(args.Source, args.Targets[0], args.Sort)
// 	response <- createEvent(result, request)
// }

// func DegreesOfSeparationHandler(request *nostr.Event, response chan nostr.Event) {
// 	args, errs := getArguments(request, true)
// 	if errs != nil {
// 		response <- createErrorEvent(errs, request)
// 		return
// 	}
// 	result := DegreesOfSeparation(args.Source, args.Targets[0], args.Sort)
// 	response <- createEvent(result, request)
// }
