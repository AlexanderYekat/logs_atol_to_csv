// чтения всех лог файлов и формирование данных по чекам
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var dirOfAtolLogs = flag.String("diratollogs", "", "директория лог фалов атол по умолчанию %appdata%\\AppData\\ATOL\\drivers10\\logs")
var clearLogsProgramm = flag.Bool("clearlogs", true, "очистить логи программы")

var LOGSDIR = "./logs/"
var RESULTSDIR = "./results/"

var filelogmap map[string]*os.File
var logsmap map[string]*log.Logger

const LEN_QUEUE_BUFFER_LOGS_STRING = 50
const LOGINFO = "info"
const LOGINFO_WITHSTD = "info_std"
const LOGERROR = "error"
const LOGSKIP_LINES = "skip_line"
const LOGOTHER = "other"
const LOG_PREFIX = "PARSING"
const Version_of_program = "2023_12_25_01"

func main() {
	var err error
	var descrError string
	runDescription := "программа версии " + Version_of_program + " парсинга лог файлов драйвера атол запущена"
	fmt.Println(runDescription)
	defer fmt.Println("программа версии " + Version_of_program + " парсинга лог файлов драйвера атол остановлена")
	fmt.Println("парсинг параметров запуска программы")
	flag.Parse()
	clearLogsDescr := fmt.Sprintf("Очистить логи программы %v", *clearLogsProgramm)
	fmt.Println(clearLogsDescr)
	fmt.Println("инициализация лог файлов программы")
	if foundedLogDir, _ := doesFileExist(LOGSDIR); !foundedLogDir {
		os.Mkdir(LOGSDIR, 0777)
	}
	if foundedLogDir, _ := doesFileExist(RESULTSDIR); !foundedLogDir {
		os.Mkdir(RESULTSDIR, 0777)
	}
	filelogmap, logsmap, descrError, err = initializationLogs(*clearLogsProgramm, LOGINFO, LOGERROR, LOGSKIP_LINES, LOGOTHER)
	defer func() {
		fmt.Println("закрытие дескрипторов лог файлов программы")
		for _, v := range filelogmap {
			if v != nil {
				//fmt.Println("close", k, v)
				v.Close()
			}
		}
	}()
	if err != nil {
		descrMistake := fmt.Sprintf("ошибка инициализации лог файлов %v", descrError)
		fmt.Fprintf(os.Stderr, descrMistake)
		log.Panic(descrMistake)
	}
	fmt.Println("лог файлы инициализированы в папке " + LOGSDIR)
	multwriterLocLoc := io.MultiWriter(logsmap[LOGINFO].Writer(), os.Stdout)
	logsmap[LOGINFO_WITHSTD] = log.New(multwriterLocLoc, LOG_PREFIX+"_"+strings.ToUpper(LOGINFO)+" ", log.LstdFlags)
	//получение директории лог файлов
	if *dirOfAtolLogs == "" {
		homeDir := UserHomeDir()
		*dirOfAtolLogs = homeDir + "\\AppData\\Roaming\\ATOL\\drivers10\\logs"

	}
	logsmap[LOGINFO].Println(runDescription)
	logsmap[LOGINFO].Println(clearLogsDescr)
	//поиск всех файлов с 9 ноября по текущий день
	//fptr10.log.2023-12-22.gz,fptr10.log.2023-12-21,fptr10.log
	listOfFilesTempr, err := listDirByReadDir(*dirOfAtolLogs)
	var listOfFiles []string
	countOfFiles := len(listOfFilesTempr)
	logsmap[LOGINFO_WITHSTD].Println("Всего лог файлов =", countOfFiles)
	//перебор всех файлов и, если необходимо, разархивирование
	logsmap[LOGINFO_WITHSTD].Println("перебор и разархивирование файлов: итоговый список файлов")
	for k, v := range listOfFilesTempr {
		currFullFileName := *dirOfAtolLogs + "\\" + v
		if filepath.Ext(v) == ".gz" {
			currFullFileName, descrError, err = DecompressFile(currFullFileName)
			if err != nil {
				descrError := fmt.Sprintf("Не удалось разархивировать лог файл %v атол c ошибкой: %v (%v)", currFullFileName, err, descrError)
				fmt.Println(descrError)
				logsmap[LOGERROR].Println(descrError)
				continue
			}
		}
		fmt.Printf("%v = %v\n", k+1, currFullFileName)
		listOfFiles = append(listOfFiles, currFullFileName)
		logsmap[LOGINFO].Printf("%v = %v\n", k+1, currFullFileName)
	}
	countOfFiles = len(listOfFiles)
	logsmap[LOGINFO_WITHSTD].Println("всего файлов лог файлов для обработки", countOfFiles)
	//перебор лог файлов и обработка
	for k, v := range listOfFiles {
		currFullFileName := v
		logsmap[LOGINFO_WITHSTD].Printf("обработка %v %v", k+1, currFullFileName)
		descrpErr, err := ReadAtolLogFile(currFullFileName)
		if err != nil {
			logsmap[LOGERROR].Println(descrpErr)
		}
	}
	//обработка лог файла
	log.Fatal("штатный выход")
}

func ReadAtolLogFile(atolLogFile string) (string, error) {
	file_atol_log, err := os.Open(atolLogFile)
	if err != nil {
		descrError := fmt.Sprintf("Не удалось открыть лог файл %v атол c ошибкой: %v", atolLogFile, err)
		logsmap[LOGERROR].Println(descrError)
		return descrError, err
	}
	scannerLogAtol := bufio.NewScanner(file_atol_log)
	var QueueLastStrings [LEN_QUEUE_BUFFER_LOGS_STRING]string
	var QueueLastCommands [LEN_QUEUE_BUFFER_LOGS_STRING]string
	var QueueLastRightCommands [LEN_QUEUE_BUFFER_LOGS_STRING]string
	//csv файл чеков
	//flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	flagsTempOpen := os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	file_checks, err := os.OpenFile(RESULTSDIR, flagsTempOpen, 0644)
	if err != nil {
		descrError := fmt.Sprintf("ошибка создания файла чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		return descrError, err
	}
	defer file_checks.Close()
	csv_checks := csv.NewWriter(file_checks)
	defer csv_checks.Flush()

	OSNChernov := ""
	OSNChist := ""
	fmt.Println(OSNChernov, OSNChist)
	lines := 0
	CashierName := ""
	CashierNameChernov := ""
	//timeMilSecondsFromLastComm = 0
	for scannerLogAtol.Scan() {
		lines++
		line := scannerLogAtol.Bytes()
		//                                                             62
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		if len(line) < 62 {
			logsmap[LOGSKIP_LINES].Println("маленький размер", "line", lines, string(line))
			continue
		}
		//                                               47
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		srCurr := line[47 : 47+len("[FiscalPrinter]")]
		if !bytes.Equal(srCurr, []byte("[FiscalPrinter]")) {
			logsmap[LOGSKIP_LINES].Println("нет [FP]", "line", lines, "slices", string(srCurr), string(line))
			continue
		}
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		//2023.11.22 13:55:59.097       T:0000DAE0 INFO  [FiscalPrinter] > LIBFPTR_PARAM_COMMODITY_NAME (65631) = "Керамогранит 600*1200 CF101 MR ГРАНИТЕ САНДРА белый (45.36/2.16)"
		//дата,время,что-то еще,тип сообщения,тип оборудования,напровление (>на кассу,<из кассы),команда, id команды, знак равно, значение команды
		//                                                             62
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		afterFiscPrint := strings.TrimSpace(string(line[62:]))
		if len(afterFiscPrint) < 9 {
			continue
		}
		currCommand := ""
		//open_shift,check_document_closed,fn_query_data,open_shift,cancel_receipt,begin_nonfiscal_document,print_text
		//operator_login,open_receipt,registration,receipt_total,payment,close_receipt
		AddLineInQueue(&QueueLastStrings, string(line))
		logsmap[LOGINFO].Println("line=", lines, string(line))
		//nextField1 nextField2
		//recv data { "e" : { "c" : 0, "d" : "Ошибок нет" }, "p" : [ { "t" : 0, "v" : 105, "n" : 65627, "s" : 0 }, { "t" : 0, "v" : 110, "n" : 65628, "s" : 0 }, { "t" : 0, "v" : 105, "n" : 65629, "s" : 0 }, { "t" : 5, "v" : { "$date" : "1970-01-18T05:54:14.400Z" }, "n" : 65590, "s" : 0 }, { "t" : 0, "v" : 105, "n" : 65692, "s" : 0 }, { "t" : 0, "v" : 120, "n" : 65693, "s" : 0 }, { "t" : 0, "v" : 261, "n" : 65753, "s" : 0 }, { "t" : 0, "v" : 110, "n" : 65819, "s" : 0 } ], "f" : null }
		nextField1 := afterFiscPrint[:4]
		nextField2 := afterFiscPrint[5:9]
		if (nextField1 == "recv") && (nextField2 == "data") {
			logsmap[LOGOTHER].Println("line", lines, afterFiscPrint)
			if strings.Contains(afterFiscPrint, "Ошибок нет") {
				AddLineInQueue(&QueueLastRightCommands, QueueLastCommands[0])
				if QueueLastRightCommands[0] == "libfptr_operator_login()" {
					CashierName = CashierNameChernov
					logsmap[LOGOTHER].Println("line", lines, CashierName)
				}
				if QueueLastRightCommands[0] == "libfptr_open_receipt()" {
					OSNChist = OSNChernov
					logsmap[LOGOTHER].Println("line", lines, OSNChist)
				}
			}
			logsmap[LOGOTHER].Println("line", lines, QueueLastCommands[0])
		}
		//libfptr_fn_query_data()
		if afterFiscPrint[:7] == "libfptr" {
			currCommand = strings.TrimSpace(afterFiscPrint)
			AddLineInQueue(&QueueLastCommands, currCommand)
		}
		if currCommand == "libfptr_open_receipt()" {
			obrStr := QueueLastStrings[2]
			afterFiscPrintObrStr := strings.TrimSpace(obrStr[62:])
			if !strings.Contains(afterFiscPrintObrStr, "1055") {
				obrStr = QueueLastStrings[1]
				afterFiscPrintObrStr = strings.TrimSpace(obrStr[62:])
			}
			//                16
			//> 1055 (1055) = 1
			OSNChernov = string(afterFiscPrintObrStr[16:])
		}
		if currCommand == "libfptr_operator_login()" {
			obrStr := QueueLastStrings[2]
			afterFiscPrintObrStr := strings.TrimSpace(obrStr[62:])
			if !strings.Contains(afterFiscPrintObrStr, "1021") {
				obrStr = QueueLastStrings[1]
				afterFiscPrintObrStr = strings.TrimSpace(obrStr[62:])
			}
			//                17
			//> 1021 (1021) = "Карнаухова Елена Геннадьевна"
			CashierNameChernov = string(afterFiscPrintObrStr[17 : len(afterFiscPrintObrStr)-1])
		}
		if currCommand == "libfptr_registration()" {
			logsmap[LOGOTHER].Println("line", lines, "libfptr_registration()0")
			obrStr := QueueLastStrings[2]
			afterFiscPrintObrStr := strings.TrimSpace(obrStr[62:])
			if !strings.Contains(afterFiscPrintObrStr, "1055") {
				obrStr = QueueLastStrings[1]
				afterFiscPrintObrStr = strings.TrimSpace(obrStr[62:])
			}
			OSNChernov = string(afterFiscPrintObrStr[16:])
		}

	}
	file_atol_log.Close()
	return "", nil
}

func AddLineInQueue(queueLastStrings *[LEN_QUEUE_BUFFER_LOGS_STRING]string, s string) error {
	for i := LEN_QUEUE_BUFFER_LOGS_STRING - 1; i > 0; i-- {
		queueLastStrings[i] = queueLastStrings[i-1]
	}
	queueLastStrings[0] = s
	return nil
}

func listDirByReadDir(path string) ([]string, error) {
	var spisFiles []string
	logsmap[LOGINFO].Printf("перебор файлов в директории %v--BEGIN\n", path)
	defer logsmap[LOGINFO].Printf("перебор файлов в директории %v--END\n", path)
	lst, err := ioutil.ReadDir(path)
	if err != nil {
		return spisFiles, err
	}
	for _, val := range lst {
		if val.IsDir() {
			continue
		}
		logsmap[LOGINFO].Println(val.Name())
		matched, err := regexp.MatchString(`fptr10\.log\.(2023\-1(1\-([0]9|[1-3][0-9])|2\-[0-9]{2}))\.gz`, val.Name())
		if !matched {
			matched, err = regexp.MatchString(`fptr10\.log\.(2023\-1(1\-([0]9|[1-3][0-9])|2\-[0-9]{2}))`, val.Name())
		}
		//matched, err := regexp.MatchString(`fptr10\.log\.2023\-(11|12)\-(09|[12][0-9]|)`, val.Name())
		if val.Name() == "fptr10.log" {
			matched = true
		}
		if err != nil {
			log.Fatal("ошибка шаблона", err)
		}
		if matched {
			if filepath.Ext(val.Name()) == ".gz" {
				//заархивированный
				newfRTemp := path + "\\" + strings.TrimSuffix(val.Name(), filepath.Ext(val.Name()))
				if f, _ := doesFileExist(newfRTemp); f {
					//fmt.Printf("файл уже %v разархивирован\n", val.Name())
					logsmap[LOGINFO].Printf("файл уже %v разархивирован\n", val.Name())
					matched = false
				}
			}
		}
		logsmap[LOGINFO].Println("matched=", matched)
		if matched {
			spisFiles = append(spisFiles, val.Name())
		}
	}
	return spisFiles, nil
}

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func initializationLogs(clearLogs bool, logstrs ...string) (map[string]*os.File, map[string]*log.Logger, string, error) {
	var reserr, err error
	reserr = nil
	filelogmapLoc := make(map[string]*os.File)
	logsmapLoc := make(map[string]*log.Logger)
	descrError := ""
	for _, logstr := range logstrs {
		filenamelogfile := logstr + "logs.txt"
		preflog := LOG_PREFIX + "_" + strings.ToUpper(logstr)
		fullnamelogfile := LOGSDIR + filenamelogfile
		//fmt.Println("logstr", logstr)
		filelogmapLoc[logstr], logsmapLoc[logstr], err = intitLog(fullnamelogfile, preflog, clearLogs)
		if err != nil {
			descrError = fmt.Sprintf("ошибка инициализации лог файла %v с ошибкой %v", fullnamelogfile, err)
			fmt.Fprintln(os.Stderr, descrError)
			reserr = err
			break
		}
	}
	return filelogmapLoc, logsmapLoc, descrError, reserr
}

func intitLog(logFile string, pref string, clearLogs bool) (*os.File, *log.Logger, error) {
	//var multwr io.Writer
	flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if clearLogs {
		flagsTempOpen = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}
	//fmt.Println("logFile=", logFile)
	f, err := os.OpenFile(logFile, flagsTempOpen, 0644)
	multwr := io.MultiWriter(f)
	//if pref == LOG_PREFIX+"_INFO" {
	//	multwr = io.MultiWriter(f, os.Stdout)
	//}
	if pref == LOG_PREFIX+"_ERROR" {
		multwr = io.MultiWriter(f, os.Stderr)
	}
	if err != nil {
		fmt.Println("Не удалось создать лог файл ", logFile, err)
		return nil, nil, err
	}
	//fmt.Println("pref=", pref)
	//fmt.Println("f=", f)
	loger := log.New(multwr, pref+" ", log.LstdFlags)
	//fmt.Println("loger=", loger)
	return f, loger, nil
}

func ScannerToReader(scanner *bufio.Scanner) io.Reader {
	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()
		for scanner.Scan() {
			writer.Write(scanner.Bytes())
		}
	}()

	return reader
}

func DecompressFile(fileName string) (string, string, error) {
	newFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	gzippedFile, err := os.Open(fileName)
	if err != nil {
		descrError := fmt.Sprintf("Не открыть разархивированный лог файл %v атол c ошибкой: %v", fileName, err)
		logsmap[LOGERROR].Println(descrError)
		return newFileName, descrError, err
	}
	gzipReader, err := gzip.NewReader(gzippedFile)
	uncompressedFile, err := os.Create(newFileName)
	if err != nil {
		descrError := fmt.Sprintf("Не удалось создать разархивированный лог файл %v атол c ошибкой: %v", fileName, err)
		fmt.Println(descrError)
		logsmap[LOGERROR].Println(descrError)
	}
	_, err = io.Copy(uncompressedFile, gzipReader)
	uncompressedFile.Close()
	gzipReader.Close()
	gzippedFile.Close()
	return newFileName, "", nil
}

func doesFileExist(fullFileName string) (found bool, err error) {
	found = false
	if _, err = os.Stat(fullFileName); err == nil {
		// path/to/whatever exists
		found = true
		//fmt.Println("true")
	} else if errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does *not* exist
		//fmt.Println("false")
	}
	//fmt.Println(err)
	return
}
