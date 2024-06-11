package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"time"
)

// User represents a user with training and availability information.
type User struct {
	Name            string          `json:"name"`
	Training        map[string]bool `json:"training"`
	DaysUnavailable []string        `json:"days_unavailable"`
}

// Info represents the structure of the info.json file.
type Info struct {
	Users            []User            `json:"users"`
	Tasks            []string          `json:"tasks"`
	TrainingRequired map[string]string `json:"training_required"`
	DaysOfWeek       []string          `json:"days_of_week"`
}

// loadInfo loads users, tasks, training requirements, and days of the week from the specified JSON file.
func loadInfo(filename string) (Info, error) {
	var info Info
	file, err := os.Open(filename)
	if err != nil {
		return info, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&info)
	return info, err
}

// loadPreviousSchedule reads the previous week's schedule from a CSV file.
func loadPreviousSchedule(filename string) (map[string]int, error) {
	userTaskCount := make(map[string]int)
	file, err := os.Open(filename)
	if err != nil {
		return userTaskCount, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return userTaskCount, err
	}

	for _, record := range records[1:] {
		user := record[1]
		userTaskCount[user]++
	}
	return userTaskCount, nil
}

// generateWeeklySchedule creates a schedule ensuring tasks are assigned to eligible users with the least tasks.
func generateWeeklySchedule(users []User, tasks []string, trainingRequired map[string]string, daysOfWeek []string, previousUserTaskCount map[string]int) (map[string]map[string]string, map[string]int) {
	schedule := make(map[string]map[string]string)
	for _, day := range daysOfWeek {
		schedule[day] = make(map[string]string)
		for _, task := range tasks {
			schedule[day][task] = ""
		}
	}

	userTaskCount := make(map[string]int)
	for _, user := range users {
		userTaskCount[user.Name] = previousUserTaskCount[user.Name]
	}

	rand.NewSource(time.Now().UnixNano())
	for day := range schedule {
		rand.Shuffle(len(tasks), func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
		for _, task := range tasks {
			var eligibleUsers []User
			for _, user := range users {
				trainingRequiredForTask := trainingRequired[task]
				if trainingRequiredForTask == "" || user.Training[trainingRequiredForTask] {
					if !isUserUnavailable(user, day) {
						eligibleUsers = append(eligibleUsers, user)
					}
				}
			}

			if len(eligibleUsers) == 0 {
				continue
			}

			leastAssignedUser := eligibleUsers[0]
			for _, user := range eligibleUsers {
				if userTaskCount[user.Name] < userTaskCount[leastAssignedUser.Name] {
					leastAssignedUser = user
				}
			}

			schedule[day][task] = leastAssignedUser.Name
			userTaskCount[leastAssignedUser.Name]++
		}
	}

	return schedule, userTaskCount
}

// isUserUnavailable checks if a user is unavailable on a given day.
func isUserUnavailable(user User, day string) bool {
	for _, unavailableDay := range user.DaysUnavailable {
		if unavailableDay == day {
			return true
		}
	}
	return false
}

// scheduleToCSV writes the schedule to a CSV file, sorting the rows by the normal order of the days of the week.
func scheduleToCSV(schedule map[string]map[string]string, daysOfWeek []string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Day"}
	for task := range schedule[daysOfWeek[0]] {
		header = append(header, task)
	}
	writer.Write(header)

	sort.Slice(daysOfWeek, func(i, j int) bool {
		order := map[string]int{"Monday": 0, "Tuesday": 1, "Wednesday": 2, "Thursday": 3, "Friday": 4}
		return order[daysOfWeek[i]] < order[daysOfWeek[j]]
	})

	for _, day := range daysOfWeek {
		tasks := schedule[day]
		record := []string{day}
		for _, task := range header[1:] {
			record = append(record, tasks[task])
		}
		writer.Write(record)
	}

	return nil
}

func main() {
	asciiArt := `                                   
         _         _     _         
 ___ ___| |_ ___ _| |_ _| |___ ___ 
|_ -|  _|   | -_| . | | | | -_|  _|
|___|___|_|_|___|___|___|_|___|_|  
                                   `
	fmt.Println(asciiArt)
	info, err := loadInfo("info.json")
	if err != nil {
		log.Fatalf("Error loading info.json: %v", err)
	}

	previousUserTaskCount, err := loadPreviousSchedule("weekly_schedule.csv")
	if err != nil {
		log.Println("No previous schedule found, starting fresh.")
	}

	weeklySchedule, userTaskCount := generateWeeklySchedule(info.Users, info.Tasks, info.TrainingRequired, info.DaysOfWeek, previousUserTaskCount)
	err = scheduleToCSV(weeklySchedule, info.DaysOfWeek, "weekly_schedule.csv")
	if err != nil {
		log.Fatalf("Error saving schedule: %v", err)
	}

	for user, count := range userTaskCount {
		fmt.Printf("%s: %d\n", user, count)
	}

	fmt.Println("Schedule generation complete! Press Enter to exit.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
