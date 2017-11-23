package offer

import (
	"github.com/guilhebl/go-offer/common/config"
	"github.com/guilhebl/go-offer/common/model"
	"github.com/guilhebl/go-offer/offer/walmart"
	"strings"
	"log"
	"github.com/guilhebl/go-worker-pool"
)

// Searches marketplace providers by keyword
func SearchOffers(m map[string]string) *model.OfferList {
	country := m["country"]
	if country == "" {
		country = model.UnitedStates
	}

	// build empty response
	capacity := config.GetIntProperty("defaultOfferListCapacity")
	list := model.NewOfferList(make([]model.Offer, 0, capacity), 1, 1, 0)

	// search providers
	providers := getProvidersByCountry(country)

	// create a slice of jobResult outputs
	jobOutputs := make([]<-chan job.JobResult, 0)

	for i := 0; i < len(providers); i++ {
		job := search(providers[i], m)
		if job != nil {
			jobOutputs = append(jobOutputs, job.ReturnChannel)
			// Push each job onto the queue.
			GetInstance().JobQueue <- *job
		}
	}

	// Consume the merged output from all jobs
	out := job.Merge(jobOutputs...)
	for r := range out {
		if r.Error == nil {
			mergeSearchResponse(list, r.Value.(*model.OfferList))
		}
	}
	return list
}

func mergeSearchResponse(list *model.OfferList, list2 *model.OfferList) {
	if list2 != nil && list2.TotalCount > 0 {
		list.List = append(list.List, list2.List...)
		list.TotalCount += list2.TotalCount
		list.PageCount += list2.PageCount
	}
}

// searches create a new Job to search in a provider that returns a OfferList channel
func search(provider string, m map[string]string) *job.Job {
	switch provider {
		case model.Walmart: return walmart.SearchOffers(m)
	}
	return nil
}

func getProvidersByCountry(country string) []string {
	switch country {
	case model.Canada:
		return strings.Split(config.GetProperty("marketplaceProvidersCanada"), ",")
	default:
		return strings.Split(config.GetProperty("marketplaceProviders"), ",")
	}
}

// Gets Product Detail from marketplace provider by Id and IdType, fetching competitors prices using UPC
func GetOfferDetail(id, idType, source, country string) *model.OfferDetail {
	det := getDetail(id, idType, source, country)

	// if product has Upc fetch competitors details in parallel using worker pool jobs
	if det != nil && det.Offer.Upc != "" {
		providers := getProvidersByCountry(country)

		// create a slice of jobResult outputs
		jobOutputs := make([]<-chan job.JobResult, 0)

		for i := 0; i < len(providers); i++ {
			if p := providers[i]; p != source {
				job := getDetailJob(det.Offer.Upc, model.Upc, providers[i], country)
				if job != nil {
					jobOutputs = append(jobOutputs, job.ReturnChannel)
					// Push each job onto the queue.
					GetInstance().JobQueue <- *job
				}
			}
		}

		// Consume the merged output from all jobs
		out := job.Merge(jobOutputs...)
		for r := range out {
			if r.Error == nil {
				// build detail item
				d := r.Value.(*model.OfferDetail)
				detItem := model.NewOfferDetailItem(
					d.Offer.PartyName,
					d.Offer.SemanticName,
					d.Offer.PartyImageFileUrl,
					d.Offer.Price,
					d.Offer.Rating,
					d.Offer.NumReviews)

				det.ProductDetailItems = append(det.ProductDetailItems, *detItem)
			}
		}
	}

	return det
}

// creates a job to fetch a product detail from a given source using id and idType and country
func getDetailJob(id, idType, source, country string) *job.Job {
	log.Printf("getDetail Job: %s, %s, %s, %s", id, idType, source, country)

	switch source {
	case model.Walmart:
		return walmart.GetDetailJob(id, idType, country)
	}

	return nil
}

// gets a product detail from a given source using id and idType and country
func getDetail(id, idType, source, country string) *model.OfferDetail {
	log.Printf("get: %s, %s, %s, %s", id, idType, source, country)

	switch source {
	case model.Walmart:
		return walmart.GetOfferDetail(id, idType, country)
	}
	return nil
}