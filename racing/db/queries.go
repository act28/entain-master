package db

const (
	racesList = "list"
	racesGet  = "get"
)

func getRaceQueries() map[string]string {
	return map[string]string{
		racesList: `
			SELECT
				id,
				meeting_id,
				name,
				number,
				visible,
				advertised_start_time
			FROM races
		`,
		racesGet: `
			SELECT
				id,
				meeting_id,
				name,
				number,
				visible,
				advertised_start_time
			FROM races
			WHERE id = ?
		`,
	}
}
