package db

const (
	eventsList = "list"
)

func getEventQueries() map[string]string {
	return map[string]string{
		eventsList: `
			SELECT
				e.id,
				e.sport_id,
				s.name AS sport_type_name,
				e.name,
				e.visible,
				e.advertised_start_time
			FROM events e
			JOIN sport_types s ON e.sport_id = s.id
		`,
	}
}
