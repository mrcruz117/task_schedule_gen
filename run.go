package main

import (
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
	Name            string
	Trainings       []string
	DaysUnavailable []string
}

// Task represents a task with required training and days on which it can be performed.
type Task struct {
	Name              string
	RequiredTrainings []string `json:"required_trainings"`
	Days              []string
	Notes             string
}

// Info represents the structure of the info.json file.
type Info struct {
	Users      []User            `json:"users"`
	Tasks      []Task            `json:"tasks"`
	Trainings  map[string]string `json:"trainings"`
	DaysOfWeek []string          `json:"days_of_week"`
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

// userHasTraining checks if a user has all the required trainings for a task.
func userHasTraining(user User, requiredTrainings []string) bool {
	trainingSet := make(map[string]bool)
	for _, training := range user.Trainings {
		trainingSet[training] = true
	}
	for _, required := range requiredTrainings {
		if !trainingSet[required] {
			return false
		}
	}
	return true
}

// isUserAvailable checks if a user is available on a given day.
func isUserAvailable(user User, day string) bool {
	for _, unavailable := range user.DaysUnavailable {
		if unavailable == day {
			return false
		}
	}
	return true
}

// shuffleUsers shuffles the users slice.
func shuffleUsers(users []User) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(users), func(i, j int) {
		users[i], users[j] = users[j], users[i]
	})
}

// loadPreviousSchedule loads the previous weekly schedule from a CSV file.
func loadPreviousSchedule(filename string) (map[string]map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	previousSchedule := make(map[string]map[string]string)
	daysOfWeek := records[0][1:]
	for _, day := range daysOfWeek {
		previousSchedule[day] = make(map[string]string)
	}

	for _, record := range records[1:] {
		task := record[0]
		for i, user := range record[1:] {
			previousSchedule[daysOfWeek[i]][task] = user
		}
	}

	return previousSchedule, nil
}

// assignTask assigns a task to the user with the least tasks who is available and has the required training,
// while avoiding assigning the same task to the same user as in the previous week where possible.
func assignTask(schedule map[string]map[string]string, users []User, task Task, day string, userTaskCount map[string]int, previousSchedule map[string]map[string]string) bool {
	sort.Slice(users, func(i, j int) bool {
		return userTaskCount[users[i].Name] < userTaskCount[users[j].Name]
	})

	previousUser := ""
	if previousSchedule != nil {
		previousUser = previousSchedule[day][task.Name]
	}

	for _, user := range users {
		if user.Name != previousUser && userHasTraining(user, task.RequiredTrainings) && isUserAvailable(user, day) {
			schedule[day][task.Name] = user.Name
			userTaskCount[user.Name]++
			return true
		}
	}

	// If no suitable user is found, allow the same user as previous week
	for _, user := range users {
		if userHasTraining(user, task.RequiredTrainings) && isUserAvailable(user, day) {
			schedule[day][task.Name] = user.Name
			userTaskCount[user.Name]++
			return true
		}
	}

	return false
}

// generateWeeklySchedule creates a schedule ensuring tasks are assigned to eligible users with the least tasks,
// while considering the previous week's schedule to avoid repeating tasks for the same users where possible.
func generateWeeklySchedule(info Info, previousSchedule map[string]map[string]string) (map[string]map[string]string, map[string]int) {
	schedule := make(map[string]map[string]string)
	userTaskCount := make(map[string]int)
	taskAssignments := make(map[string]string)

	for _, day := range info.DaysOfWeek {
		schedule[day] = make(map[string]string)
	}

	// Handle special tasks first
	for _, task := range info.Tasks {
		if task.Notes == "same person all week" {
			shuffleUsers(info.Users)
			for _, user := range info.Users {
				if userHasTraining(user, task.RequiredTrainings) {
					for _, day := range info.DaysOfWeek {
						schedule[day][task.Name] = user.Name
					}
					taskAssignments[task.Name] = user.Name
					userTaskCount[user.Name] += len(info.DaysOfWeek)
					break
				}
			}
		}
	}

	// Handle EOD Reports and Late Person Tasks together
	for _, task := range info.Tasks {
		if task.Notes == "whoever has EOD has late person tasks" {
			for _, day := range task.Days {
				shuffleUsers(info.Users)
				for _, user := range info.Users {
					if userHasTraining(user, task.RequiredTrainings) && isUserAvailable(user, day) {
						schedule[day][task.Name] = user.Name
						schedule[day]["Late Person Tasks"] = user.Name
						userTaskCount[user.Name] += 2
						break
					}
				}
			}
		}
	}

	// Assign remaining tasks
	for _, task := range info.Tasks {
		if _, exists := taskAssignments[task.Name]; exists {
			continue
		}
		if task.Notes == "whoever has EOD has late person tasks" {
			continue
		}
		for _, day := range task.Days {
			assigned := assignTask(schedule, info.Users, task, day, userTaskCount, previousSchedule)
			if !assigned {
				log.Printf("No user available for task %s on %s", task.Name, day)
			}
		}
	}

	return schedule, userTaskCount
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

	header := append([]string{"Task"}, daysOfWeek...)
	writer.Write(header)

	taskSet := make(map[string]bool)
	for _, dayTasks := range schedule {
		for task := range dayTasks {
			taskSet[task] = true
		}
	}

	tasks := make([]string, 0, len(taskSet))
	for task := range taskSet {
		tasks = append(tasks, task)
	}
	sort.Strings(tasks)

	for _, task := range tasks {
		record := []string{task}
		for _, day := range daysOfWeek {
			record = append(record, schedule[day][task])
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

	// exePath, err := os.Executable()
	// if err != nil {
	// 	log.Fatalf("Error getting executable path: %v", err)
	// }
	// exeDir := filepath.Dir(exePath)

	// err = os.Chdir(exeDir)
	// if err != nil {
	// 	log.Fatalf("Error changing working directory: %v", err)
	// }

	info, err := loadInfo("info.json")
	if err != nil {
		log.Fatalf("Error loading info.json: %v", err)
	}

	var previousSchedule map[string]map[string]string
	if _, err := os.Stat("previous_weekly_schedule.csv"); err == nil {
		previousSchedule, err = loadPreviousSchedule("previous_weekly_schedule.csv")
		if err != nil {
			log.Printf("Error loading previous schedule: %v", err)
		}
	}

	schedule, _ := generateWeeklySchedule(info, previousSchedule)

	err = scheduleToCSV(schedule, info.DaysOfWeek, "weekly_schedule.csv")
	if err != nil {
		log.Fatalf("Error saving schedule: %v", err)
	}

	// Print the number of tasks per person
	// fmt.Println("Number of tasks per person:")
	// for user, count := range userTaskCount {
	// 	fmt.Printf("%s: %d tasks\n", user, count)
	// }
	fmt.Println("\nSchedule generation complete! Check the weekly_schedule.csv file. Press Enter to exit.")
	fmt.Scanln()

}
