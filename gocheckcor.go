// чтения всех лог файлов и формирование данных по чекам
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"strings"

	//"log/syslog"

	"os"
)

var dirOfAtolLogs = flag.String("diratollogs", "./atollogs/", "директория лог фалов атол")
var clearLogsProgramm = flag.Bool("clearlogs", true, "очистить логи программы")
var LOGFILE_INFO = "./logs/infologs.txt"
var LOGFILE_ERROR = "./logs/errorlogs.txt"
var LOGFILE_OTHER = "./logs/otherlogs.txt"

const LEN_QUEUE_BUFFER_LOGS_STRING = 50

func main() {
	fmt.Println("go check cor ver 2023 12 24 _ 01")
	flag.Parse()
	fmt.Println("Директория лог фалов атол =", *dirOfAtolLogs)
	fmt.Println("Очитсть логи программы =", *clearLogsProgramm)
	/*ex, err := os.Executable()
	if err != nil {
		fmt.Println("Ошибка Executable ", err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println("Директория файла программы=", exPath)*/
	flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if *clearLogsProgramm {
		flagsTempOpen = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}
	fOtherLogs, err := os.OpenFile(LOGFILE_OTHER, flagsTempOpen, 0644)
	if err != nil {
		fmt.Println("Не удалось создать лог файл ", LOGFILE_OTHER, err)
		return
	}
	otherLog := log.New(fOtherLogs, "PARSING_OTHER ", log.LstdFlags)
	defer fOtherLogs.Close()
	fInfoLogs, err := os.OpenFile(LOGFILE_INFO, flagsTempOpen, 0644)
	if err != nil {
		fmt.Println("Не удалось создать лог файл ", LOGFILE_INFO, err)
		return
	}
	defer fInfoLogs.Close()
	infoLog := log.New(fInfoLogs, "PARSING_INFO ", log.LstdFlags)
	fErrorLogs, err := os.OpenFile(LOGFILE_ERROR, flagsTempOpen, 0644)
	if err != nil {
		fmt.Println("Не удалось создать лог файл ", LOGFILE_ERROR, err)
		return
	}
	defer fErrorLogs.Close()
	errorLog := log.New(fErrorLogs, "PARSING_ERROR ", log.LstdFlags|log.Lshortfile)
	//errorLog.Fatal("ошибка")
	//получение директории лог файлов
	//поиск всех файлов с 9 ноября по текущий день
	//перебор всех файлов
	//обработка лог файла
	file_atol_log, err := os.Open("./atollogs/fptr10.log")
	if err != nil {
		fmt.Println("Не удалось открыть файл: \"./atollogs/fptr10.log с ошибкой", err)
		errorLog.Println("Ошибка открытия файл лога атол", err)
		panic(err)
	}
	scannerLogAtol := bufio.NewScanner(file_atol_log)
	var QueueLastStrings [LEN_QUEUE_BUFFER_LOGS_STRING]string
	var QueueLastCommands [LEN_QUEUE_BUFFER_LOGS_STRING]string
	var QueueLastRightCommands [LEN_QUEUE_BUFFER_LOGS_STRING]string
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
		if len(line) < 62 {
			errorLog.Println("маленький размер", "line", lines, string(line))
			continue
		}
		//otherLog.Println("line", lines, "slices", string(line))
		//otherLog.Println("line", lines, "slices", string(line[:62]))
		srCurr := line[47 : 47+len("[FiscalPrinter]")]
		//otherLog.Println("line", lines, string(srCurr))
		//fmt.Println()
		//fmt.Println(string(line))
		//fmt.Println(string(srCurr))
		if !bytes.Equal(srCurr, []byte("[FiscalPrinter]")) {
			errorLog.Println("нет [FP]", "line", lines, "slices", string(srCurr), string(line))
			continue
		}
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		//2023.11.22 13:55:59.097       T:0000DAE0 INFO  [FiscalPrinter] > LIBFPTR_PARAM_COMMODITY_NAME (65631) = "Керамогранит 600*1200 CF101 MR ГРАНИТЕ САНДРА белый (45.36/2.16)"
		//дата,время,что-то еще,тип сообщения,тип оборудования,напровление (>на кассу,<из кассы),команда, id команды, знак равно, значение команды
		afterFiscPrint := strings.TrimSpace(string(line[62:]))
		if len(afterFiscPrint) < 9 {
			continue
		}
		currCommand := ""
		//open_shift,check_document_closed,fn_query_data,open_shift,cancel_receipt,begin_nonfiscal_document,print_text
		//operator_login,open_receipt,registration,receipt_total,payment,close_receipt
		AddLineInQueue(&QueueLastStrings, string(line))
		infoLog.Println("line=", lines, string(line))
		nextField1 := afterFiscPrint[:4]
		nextField2 := afterFiscPrint[5:9]
		if (nextField1 == "recv") && (nextField2 == "data") {
			otherLog.Println("line", lines, afterFiscPrint)
			if strings.Contains(afterFiscPrint, "Ошибок нет") {
				AddLineInQueue(&QueueLastRightCommands, QueueLastCommands[0])
				if QueueLastRightCommands[0] == "libfptr_operator_login()" {
					CashierName = CashierNameChernov
					otherLog.Println("line", lines, CashierName)
				}
				if QueueLastRightCommands[0] == "libfptr_open_receipt()" {
					OSNChist = OSNChernov
					otherLog.Println("line", lines, OSNChist)
				}
			}
			otherLog.Println("line", lines, QueueLastCommands[0])
		}
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
			OSNChernov = string(afterFiscPrintObrStr[16:])
		}
		if currCommand == "libfptr_operator_login()" {
			obrStr := QueueLastStrings[2]
			afterFiscPrintObrStr := strings.TrimSpace(obrStr[62:])
			if !strings.Contains(afterFiscPrintObrStr, "1021") {
				obrStr = QueueLastStrings[1]
				afterFiscPrintObrStr = strings.TrimSpace(obrStr[62:])
			}
			CashierNameChernov = string(afterFiscPrintObrStr[17 : len(afterFiscPrintObrStr)-1])
		}
		if currCommand == "libfptr_registration()" {
			otherLog.Println("line", lines, "libfptr_registration()0")
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
}

func AddLineInQueue(queueLastStrings *[LEN_QUEUE_BUFFER_LOGS_STRING]string, s string) error {
	for i := LEN_QUEUE_BUFFER_LOGS_STRING - 1; i > 0; i-- {
		queueLastStrings[i] = queueLastStrings[i-1]
	}
	queueLastStrings[0] = s
	return nil
}
