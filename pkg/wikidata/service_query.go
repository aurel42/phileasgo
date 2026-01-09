package wikidata

import (
	"fmt"
)

func buildCheapQuery(lat, lon float64, radius string) string {
	// Radius passed dynamically
	if radius == "" {
		radius = "9.8" // Fallback
	}

	// CHEAP QUERY:
	// - No Labels
	// - No Subquery for titles
	// - Just QID, Sitelinks, Dimensions, Instances
	return fmt.Sprintf(`SELECT DISTINCT ?item ?lat ?lon ?sitelinks 
            (GROUP_CONCAT(DISTINCT ?instance_of_uri; separator=",") AS ?instances) 
            ?area ?height ?length ?width
        WHERE { 
            SERVICE wikibase:around { 
                ?item wdt:P625 ?location . 
                bd:serviceParam wikibase:center "Point(%f %f)"^^geo:wktLiteral . 
                bd:serviceParam wikibase:radius "%s" . 
            } 
            ?item p:P625/psv:P625 [ wikibase:geoLatitude ?lat ; wikibase:geoLongitude ?lon ] . 
            
            OPTIONAL { ?item wdt:P31 ?instance_of_uri . } 
            OPTIONAL { ?item wikibase:sitelinks ?sitelinks . } 
            OPTIONAL { ?item wdt:P2046 ?area . }
            OPTIONAL { ?item wdt:P2048 ?height . }
            OPTIONAL { ?item wdt:P2043 ?length . }
            OPTIONAL { ?item wdt:P2049 ?width . }
            
            FILTER(?sitelinks > 0)
        } 
        GROUP BY ?item ?lat ?lon ?sitelinks ?area ?height ?length ?width
        ORDER BY DESC(?sitelinks) 
        LIMIT 500`, lon, lat, radius)
}
