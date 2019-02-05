package spellcheck

import (
	// "fmt"
	"log"
	"bufio"
	"os"
	"os/user"
	"regexp"
	"strings"
	// "time"
	"strconv"
	"encoding/gob"
)

var WordModel  map[string]int
var ErrorModel map[string]map[string]int
var wordMapFile  string = "/.spellWordMap.gob"
var errorMapFile string = "/.spellErrorMap.gob"

// Remove duplicates in a slice 
func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}

	return result
}

// Function that gets the maximum probable string using given models
func max(origWord string, words []string) string {
	var maxProb float64
	var bestString string

    for i := range words {
		if words[i] == origWord {
			maxProb = float64(WordModel[words[i]])
			bestString = words[i]
		} else {
			sum := 0
			for j := range ErrorModel[words[i]] {
				sum += ErrorModel[words[i]][j]
			}

			maxProb = float64(WordModel[words[i]]) * (float64(ErrorModel[words[i]][origWord])/float64(sum))
			bestString = words[i]
		}

        break
	}
	
    for i := range words {
		if words[i] == origWord {
			tempProb := float64(WordModel[words[i]])
			if tempProb > maxProb {
				maxProb = tempProb
				bestString = words[i]
			}
		} else {
			sum := 0
			for j := range ErrorModel[words[i]] {
				sum += ErrorModel[words[i]][j]
			}

			tempProb := float64(WordModel[words[i]]) * (float64(ErrorModel[words[i]][origWord])/float64(sum))
			if tempProb > maxProb {
				maxProb = tempProb
				bestString = words[i]
			}
		}
    }
    return bestString
}

// Reading words file and storing their frequencies in the map
func trainWordsModel(words_training_data string) map[string]int {
	file1, err := os.Open(words_training_data)
	if err != nil {
		log.Fatal(err)
	}
	defer file1.Close()

	scanner 	  := bufio.NewScanner(file1)
	NWORDS 		  := make(map[string]int)
	wordPattern   := regexp.MustCompile("[a-z]+")
	numberPattern := regexp.MustCompile("\\d+")
	for scanner.Scan() {
		w := wordPattern.FindString(scanner.Text())
		n := numberPattern.FindString(scanner.Text())
		NWORDS[w], err = strconv.Atoi(n)
		NWORDS[strings.Title(w)] = NWORDS[w]

		if err!=nil {
			log.Fatal(err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return NWORDS
}

// Training the error model
func trainErrorModel(error_training_data string) map[string]map[string]int {
	file, err := os.Open(error_training_data)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	errorScanner  := bufio.NewScanner(file)
	EWORDS 		  := make(map[string]map[string]int)
	wordPattern   := regexp.MustCompile("[^:]+:")
	errorPattern  := regexp.MustCompile("\\s[a-z]+")
	
	for errorScanner.Scan() {
		if(len(errorScanner.Text()) > 0) {
			temp := wordPattern.FindString(errorScanner.Text())
			origWord   := temp[:len(temp)-1]
			errorWords := errorPattern.FindAllString(errorScanner.Text(), -1)
			
			EWORDS[origWord] = make(map[string]int)
			for i := range errorWords {
				errorWords[i] = errorWords[i][1:]
				EWORDS[origWord][errorWords[i]]++
			}
		}
	}

	if err := errorScanner.Err(); err != nil {
		log.Fatal(err)
	}
	
	return EWORDS
}

// Function to return all possible strings having edit distance of 1 from word
func edits1(word string, ch chan string) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	type Pair struct{a, b string}
	var splits []Pair

	// Creating a list of all tuples formed by splitting the word at all possible positions
	for i := 0; i < len(word) + 1; i++ {
		splits = append(splits, Pair{word[:i], word[i:]}) }

	for _, s := range splits {
		// Words formed by deleting one character from original word
		if len(s.b) > 0 { ch <- s.a + s.b[1:] }

		// Words formed by transposing two characters in original word
		if len(s.b) > 1 { ch <- s.a + string(s.b[1]) + string(s.b[0]) + s.b[2:] }

		// Words formed by replacing one character in original word
		for _, c := range alphabet { if len(s.b) > 0 { ch <- s.a + string(c) + s.b[1:] }}

		// Words formed by adding one character to original word
		for _, c := range alphabet { ch <- s.a + string(c) + s.b }
	}
}

// Function to return all possible strings having edit distance of 2 from word
func edits2(word string, ch chan string) {
	ch1 := make(chan string, 1024*1024)
	go func() { edits1(word, ch1); ch1 <- "" }()
	for e1 := range ch1 {
		if e1 == "" { break }
		go func(){ edits1(e1, ch) }()
	}
}

// Function to return the best candidate present in model formed by given edits distance function
func best(origWord string, edits func(string, chan string)) string {
	ch := make(chan string, 1024*1024)
	go func() { edits(origWord, ch); ch <- "" }()

	var maxProb float64 = 0
	var bestString string = ""

    for word := range ch {
		if word == "" { break }

		if word == origWord {
			tempProb := float64(WordModel[word]) 

			if tempProb > maxProb {
				maxProb = tempProb
				bestString = word
			}
		} else {
			sum := 0
			for j := range ErrorModel[word] {
				sum += ErrorModel[word][j]
			}

			tempProb := float64(WordModel[word]) * (float64(ErrorModel[word][origWord])/float64(sum))
			if tempProb > maxProb {
				maxProb = tempProb
				bestString = word
			}
		}
	}

	if len(bestString) == 0 {
		go func() { edits(origWord, ch); ch <- "" }()
		maxFreq := 0
		for word := range ch {
			if word == "" { break }
			if freq, present := WordModel[word]; present && freq > maxFreq {
				maxFreq, bestString = freq, word
			}
		}
	}

	return bestString
}

func correct(word string) string {
	var possibleWords []string

	if _, present := WordModel[word]; present {
		possibleWords = append(possibleWords, word)	
	}
	if correction := best(word, edits1); correction != "" {
		possibleWords = append(possibleWords, correction) 
	}
	if correction := best(word, edits2); correction != "" { 
		possibleWords = append(possibleWords, correction) 
	}

	// If no word at edit distance of 1 or 2 matches
	if len(possibleWords) == 0 {
		return word
	}

	// Removing duplicates in possibleWords
	possibleWords = removeDuplicates(possibleWords)

	return max(word, possibleWords)
}

func helper(word string, ch chan string) {
	re   := regexp.MustCompile(`([^!?,.;]+)`)
	ch <- re.ReplaceAllStringFunc(word, func(m string) string {
				return correct(m)
			})
}

func Correctsentence(sentence string) string {

	s1 := strings.Split(sentence, "\n")
	for i := range s1 {
		s2 := strings.Split(s1[i], "\t")
		for j := range s2 {
			s3 := strings.Split(s2[j], " ")
			channels := make([]chan string, len(s3))
			for k := range s3 {
				channels[k] = make(chan string)
				go helper(s3[k], channels[k])
			}

			for k := range s3 {
				s3[k] = <-channels[k]
			}

			s2[j] = strings.Join(s3, " ")
		}
		s1[i] = strings.Join(s2, "\t")
	}
	correctedSentence := strings.Join(s1, "\n")
	// re   := regexp.MustCompile(`([^!?,.\n\t ]+)`)
	// correctedSentence := re.ReplaceAllStringFunc(sentence, func(m string) string {
	// 						return correct(m)
	// 					})
	return correctedSentence
}

func Init(){
	usr, err := user.Current()
    if err != nil {
        log.Fatal( err )
    }
	wordMapFile = usr.HomeDir + wordMapFile
 	errorMapFile = usr.HomeDir + errorMapFile

	// Load WordMap
	if _, err := os.Stat(wordMapFile); err == nil {
		file, err := os.Open(wordMapFile)
		defer file.Close()

		if err == nil {
			decoder := gob.NewDecoder(file)
			err = decoder.Decode(&WordModel)
		} 
		if err != nil {
			log.Fatal(err)
		}
	  
	} else if os.IsNotExist(err) {
		WordModel = trainWordsModel("words.txt")
		file, err := os.Create(wordMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(WordModel)
		} else {
			log.Fatal(err)
		}
		 
	} else {
		log.Fatal(err)
	}

	// Load ErrorMap
	if _, err := os.Stat(errorMapFile); err == nil {
		file, err := os.Open(errorMapFile)
		defer file.Close()

		if err == nil {
			decoder := gob.NewDecoder(file)
			err = decoder.Decode(&ErrorModel)
		} 
		if err != nil {
			log.Fatal(err)
		}
	  
	} else if os.IsNotExist(err) {
		ErrorModel = trainErrorModel("errors.txt")
		file, err := os.Create(errorMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(ErrorModel)
		} else {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}
}

// Save maps in file
func SaveMaps() {
	usr, err := user.Current()
    if err != nil {
        log.Fatal( err )
    }
	wordMapFile = usr.HomeDir + wordMapFile
	errorMapFile = usr.HomeDir + errorMapFile
	 
	// Load WordMap
	if _, err := os.Stat(wordMapFile); err == nil {
		file, err := os.Open(wordMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(WordModel)
		} else {
			log.Fatal(err)
		}
	  
	} else if os.IsNotExist(err) {
		WordModel = trainWordsModel("words.txt")
		file, err := os.Create(wordMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(WordModel)
		} else {
			log.Fatal(err)
		}
		 
	} else {
		log.Fatal(err)
	}

	// Load ErrorMap
	if _, err := os.Stat(errorMapFile); err == nil {
		file, err := os.Open(errorMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(ErrorModel)
		} else {
			log.Fatal(err)
		}
	  
	} else if os.IsNotExist(err) {
		ErrorModel = trainErrorModel("errors.txt")
		file, err := os.Create(errorMapFile)
		defer file.Close()

		if err == nil {
			encoder := gob.NewEncoder(file)
			encoder.Encode(ErrorModel)
		} else {
			log.Fatal(err)
		}

	} else {
		log.Fatal(err)
	}
}

// func main() {
// 	Init()
// 	startTime := time.Now()
// 	fmt.Println(Correctsentence("Speling Errurs IN somethink. Whutever; unusuel misteakes?"))
// 	fmt.Printf("Time: %v\n", time.Now().Sub(startTime))
// }