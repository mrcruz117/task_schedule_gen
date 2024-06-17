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
	Name            string   `json:"name"`
	Trainings       []string `json:"trainings"`
	DaysUnavailable []string `json:"days_unavailable"`
}

// Task represents a task with required training and days on which it can be performed.
type Task struct {
	Name              string   `json:"name"`
	RequiredTrainings []string `json:"required_trainings"`
	Days              []string `json:"days"`
	Notes             string   `json:"notes"`
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
	rand.NewSource(time.Now().UnixNano())
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

type WeightedUser struct {
	User   User
	Weight int
}

func assignTask(
	schedule map[string]map[string]string,
	users []User,
	task Task,
	day string,
	userTaskCount map[string]int,
	previousSchedule map[string]map[string]string) bool {

	// Shuffle the users slice normally
	rand.Shuffle(len(users), func(i, j int) { users[i], users[j] = users[j], users[i] })

	// Determine the previous day correctly
	days := getSortedKeys(schedule) // Ensure this function returns the days in correct order
	previousDayIndex := -1
	for i, d := range days {
		if d == day {
			previousDayIndex = i - 1
			break
		}
	}
	previousDay := ""
	if previousDayIndex >= 0 {
		previousDay = days[previousDayIndex]
	}

	previousUser := ""
	if previousSchedule != nil && previousDay != "" {
		if _, exists := previousSchedule[previousDay]; exists {
			previousUser = previousSchedule[previousDay][task.Name]
		}
	}

	// Filter users who meet the criteria
	var eligibleUsers []User
	for _, user := range users {
		// Skip if the user was assigned the same task on the previous day in the current schedule
		if previousDay != "" && schedule[previousDay][task.Name] == user.Name {
			continue
		}

		// Skip if the user was assigned the same task on the same day last week
		if previousSchedule != nil {
			if prevUser, exists := previousSchedule[day][task.Name]; exists && prevUser == user.Name {
				continue
			}
		}

		// Ensure the user is not the same as the previous week's user and has the required training and availability
		if user.Name != previousUser && userHasTraining(user, task.RequiredTrainings) && isUserAvailable(user, day) {
			eligibleUsers = append(eligibleUsers, user)
		}
	}

	if len(eligibleUsers) == 0 {
		return false // No suitable user found
	}

	// Find the minimum task count among eligible users
	minTaskCount := userTaskCount[eligibleUsers[0].Name]
	for _, user := range eligibleUsers {
		if userTaskCount[user.Name] < minTaskCount {
			minTaskCount = userTaskCount[user.Name]
		}
	}

	// Calculate the range of task counts to consider
	taskCounts := make([]int, len(eligibleUsers))
	for i, user := range eligibleUsers {
		taskCounts[i] = userTaskCount[user.Name]
	}
	sort.Ints(taskCounts)
	rangeEnd := minTaskCount + int(float64(len(eligibleUsers))*0.3) // Use the 10th percentile as the range

	// Filter users who have the minimum task count or within the calculated range
	var leastLoadedUsers []User
	for _, user := range eligibleUsers {
		if userTaskCount[user.Name] <= rangeEnd {
			if user.Name == "Sophia" && userTaskCount[user.Name] == 9 {
				fmt.Println("Sophia has 9 tasks")
				continue
			}
			leastLoadedUsers = append(leastLoadedUsers, user)
		}
	}

	if len(leastLoadedUsers) == 0 {
		return false // No suitable user found
	}

	// Randomly select from the least loaded users
	selectedUser := leastLoadedUsers[rand.Intn(len(leastLoadedUsers))]

	// Assign the task to the selected user
	schedule[day][task.Name] = selectedUser.Name
	userTaskCount[selectedUser.Name]++
	return true
}

// Helper function to get sorted keys of a map
func getSortedKeys(m map[string]map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

	// Handle EOD Reports and Late Person Tasks
	for _, task := range info.Tasks {
		if task.Name == "EOD Reports" {
			for _, day := range task.Days {
				assigned := assignTask(schedule, info.Users, task, day, userTaskCount, previousSchedule)
				if assigned {
					schedule[day]["Late Person Tasks"] = schedule[day][task.Name]
					userTaskCount[schedule[day][task.Name]]++
				} else {
					log.Printf("No user available for task %s on %s", task.Name, day)
				}
			}
		}
	}
	// Assign remaining tasks
	for _, task := range info.Tasks {
		// Check if the task has already been assigned
		if _, exists := taskAssignments[task.Name]; exists {
			continue // Skip this task as it's already been handled
		}
		for _, day := range task.Days {
			if _, exists := schedule[day][task.Name]; exists {
				continue // Skip this task as it's already been handled
			}
			assigned := assignTask(schedule, info.Users, task, day, userTaskCount, previousSchedule)
			if !assigned {
				log.Printf("No user available for task %s on %s", task.Name, day)
			} else {
				// If the task is successfully assigned, mark it as handled
				taskAssignments[task.Name] = schedule[day][task.Name]
			}
		}
	}

	// // check eod and late person tasks by day
	// for _, day := range info.DaysOfWeek {
	// 	fmt.Println(day)
	// 	fmt.Println("EOD Reports", schedule[day]["EOD Reports"])
	// 	fmt.Println("Late Person Tasks", schedule[day]["Late Person Tasks"])
	// }

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
			if name, ok := schedule[day][task]; ok {
				record = append(record, name)
			} else {
				record = append(record, "")
			}
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

	schedule, userTaskCount := generateWeeklySchedule(info, previousSchedule)

	err = scheduleToCSV(schedule, info.DaysOfWeek, "weekly_schedule.csv")
	if err != nil {
		log.Fatalf("Error saving schedule: %v", err)
	}

	// Print the number of tasks per person
	fmt.Println("Number of tasks per person:")
	for user, count := range userTaskCount {
		fmt.Printf("%s: %d tasks\n", user, count)
	}
	fmt.Println("\nSchedule generation complete! Check the weekly_schedule.csv file. Press Enter to exit.")
	// fmt.Scanln()

}
