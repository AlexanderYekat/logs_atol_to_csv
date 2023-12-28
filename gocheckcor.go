// чтения всех лог файлов и формирование данных по чекам
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var dirOfAtolLogs = flag.String("diratollogs", "", "директория лог фалов атол по умолчанию %appdata%\\AppData\\ATOL\\drivers10\\logs")
var clearLogsProgramm = flag.Bool("clearlogs", true, "очистить логи программы")

var LOGSDIR = "./logs/"

//var RESULTSDIR = "./results/"

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

const FILE_NAME_BASE_CHECKS_HEADER = "checks_header.csv"
const FILE_NAME_BASE_CHECKS_POSITIONS = "checks_posits.csv"

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
	//if foundedLogDir, _ := doesFileExist(RESULTSDIR); !foundedLogDir {
	//	os.Mkdir(RESULTSDIR, 0777)
	//}
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
		fmt.Fprint(os.Stderr, descrMistake)
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
	if err != nil {
		logsmap[LOGERROR].Printf("ошибка поиска лог файлов атол в директории %v c ошибкой %v", *dirOfAtolLogs, err)
	}
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
				logsmap[LOGERROR].Println(descrError)
				continue
			}
		}
		//fmt.Printf("%v = %v\n", k+1, currFullFileName)
		listOfFiles = append(listOfFiles, currFullFileName)
		logsmap[LOGINFO_WITHSTD].Printf("%v = %v\n", k+1, currFullFileName)
	}
	countOfFiles = len(listOfFiles)
	//инициализация базы чеков
	logsmap[LOGINFO_WITHSTD].Println("инициализация базы чеков")
	//очистка, создание переменной записи
	//os.Truncate(FILE_NAME_BASE_CHECKS_HEADER, 0)
	//csv файл чеков
	//flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	logsmap[LOGINFO_WITHSTD].Println("инициализация таблицы шапки чеков")
	flagsTempOpen := os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	file_checks_header, err := os.OpenFile(FILE_NAME_BASE_CHECKS_HEADER, flagsTempOpen, 0644)
	if err != nil {
		descrError := fmt.Sprintf("ошибка создания файла чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		log.Panic("ошибка инициализации реузльтрующего файла чеков", descrError)
	}
	defer file_checks_header.Close()
	csv_checks_header := csv.NewWriter(file_checks_header)
	csv_checks_header.Comma = ';'
	fieldsCsvHeader := []string{"deviceId", "factoryNum", "regNum", "fn", "ffdstr", "ffdint", "ffdtag",
		"INN", "name", "place", "cashier", "FD", "FP", "dateandtime", "date", "time", "OSN",
		"typeReciept", "typeofcorrection", "datecorrection", "documNumofCorrection", "thereMarks",
		"summ", "nal", "beznal", "avance", "credit", "predstav", "6pay", "7pay", "8pay", "9pay", "10pay"}
	err = csv_checks_header.Write(fieldsCsvHeader)
	if err != nil {
		descrError := fmt.Sprintf("ошибка записи заголовка файла шапки чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		log.Panic(descrError)
	}
	defer csv_checks_header.Flush()

	logsmap[LOGINFO_WITHSTD].Println("инициализация таблицы позиций чеков")
	flagsTempOpen = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	file_checks_positions, err := os.OpenFile(FILE_NAME_BASE_CHECKS_POSITIONS, flagsTempOpen, 0644)
	if err != nil {
		descrError := fmt.Sprintf("ошибка создания файла позиций чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		log.Panic("ошибка инициализации реузльтрующего файла позиций чеков", descrError)
	}
	defer file_checks_positions.Close()
	csv_checks_positions := csv.NewWriter(file_checks_positions)
	csv_checks_positions.Comma = ';'
	fieldsCsvPosit := []string{"name", "price", "quantity", "mera", "amount", "predm", "sposob",
		"stavka", "buyername", "buyerinn", "agent", "addressagent", "innagent", "nameagent",
		"telephagent", "telephoperpriem", "telephoperperev", "innspl", "namespl",
		"makr", "statusofmark", "validationofmark", "processingmodeofmark"}
	err = csv_checks_positions.Write(fieldsCsvPosit)
	if err != nil {
		descrError := fmt.Sprintf("ошибка записи заголовка файла позиций чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		log.Panic(descrError)
	}
	defer csv_checks_positions.Flush()

	//перебор лог файлов и обработка
	logsmap[LOGINFO_WITHSTD].Println("всего файлов лог файлов для обработки", countOfFiles)
	for k, currFullFileName := range listOfFiles {
		logsmap[LOGINFO_WITHSTD].Printf("обработка %v из %v %v", k+1, countOfFiles, currFullFileName)
		descrpErr, err := ReadAtolLogFile(currFullFileName, csv_checks_header, csv_checks_positions)
		if err != nil {
			errorDescr := fmt.Sprintf("ошибка (%v) обработки лог файла %v атол", descrpErr, currFullFileName)
			logsmap[LOGERROR].Println(errorDescr)
		}
	}
	//обработка лог файла
	log.Fatal("штатный выход")
}

func lastAssenatialActWasCloseReceipt(lastActions [LEN_QUEUE_BUFFER_LOGS_STRING]string) (res bool) {
	res = false
	for i := 0; i < LEN_QUEUE_BUFFER_LOGS_STRING; i++ {
		breakFromLoop := false
		switch lastActions[i] {
		case "close_receipt":
			res = true
			breakFromLoop = true
		case "open_shift", "cancel_receipt", "operator_login", "open_receipt", "registration",
			"receipt_total", "payment":
			res = false
			breakFromLoop = true
		}
		if breakFromLoop {
			break
		}
	}
	return res
}

// registrationMap, paymentMap, operatorLoginMap, openReceiptMap, receiptTotalMap
func processingResultOfCommand(namelogfile string, numLine int, deviceId string, deviceMap map[string]string,
	command string, queueLastActs [LEN_QUEUE_BUFFER_LOGS_STRING]string,
	parametersIn, parametersOut map[string]string,
	registrationMap map[string]map[string]string,
	operatorLoginMap, openReceiptMap, paymentMap map[string]string, receiptTotal *string) (map[string]string,
	map[string]map[string]string, string, error) {
	//check_document_closed,fn_query_data,open_shift,cancel_receipt,
	//begin_nonfiscal_document,print_text,operator_login,open_receipt,registration,
	//receipt_total,payment,close_receipt,query_data
	var reslHeaderReciept map[string]string
	var reslRegistrReciept map[string]map[string]string
	logsmap[LOGOTHER].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
		"zavNum", deviceMap["zavNum"], "оброботка команды", command, "с входными параметрами", parametersIn, "и выходными", parametersOut)
	switch command {
	case "cancel_receipt":
		//очищаем информацию об кассирах, осн, типа чека, регистрациях, оплатах и суммах
		clearParametersOfCommand(operatorLoginMap)
		clearParametersOfCommand(openReceiptMap)
		clearParametersOfCommand(paymentMap)
		*receiptTotal = ""
		for kTemp := range registrationMap {
			clearParametersOfCommand(registrationMap[kTemp])
		}
		for kTemp := range registrationMap {
			delete(registrationMap, kTemp)
		}
	case "operator_login": //запоминаем кассира
		fmt.Println(parametersIn["1021"]) //имя кассира
		fmt.Println(parametersIn["1203"]) //ИНН кассира
		for kTemp, vTemp := range parametersIn {
			operatorLoginMap[kTemp] = vTemp
		}
	case "open_receipt": //запоминаем тип чека и систему налогообложения
		fmt.Println(parametersIn["1055"])                       //система налогообложкния
		fmt.Println(parametersIn["1187"])                       //место расчетов
		fmt.Println(parametersIn["LIBFPTR_PARAM_RECEIPT_TYPE"]) //тип чека
		for kTemp, vTemp := range parametersIn {
			openReceiptMap[kTemp] = vTemp
		}
		//LIBFPTR_RT_CLOSED = 0,
		//LIBFPTR_RT_SELL = 1,
		//LIBFPTR_RT_SELL_RETURN = 2,
		//LIBFPTR_RT_SELL_CORRECTION = 7,
		//LIBFPTR_RT_SELL_RETURN_CORRECTION = 8,
		//LIBFPTR_RT_BUY = 4,
		//LIBFPTR_RT_BUY_RETURN = 5,
		//LIBFPTR_RT_BUY_CORRECTION = 9,
		//LIBFPTR_RT_BUY_RETURN_CORRECTION = 10,
	case "registration":
		//запоминаем позицию, если чек будет закрыт, то запишем эту ифнформацию в файл
		fmt.Println(parametersIn["LIBFPTR_PARAM_COMMODITY_NAME"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_POSITION_SUM"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_PRICE"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_QUANTITY"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_TAX_TYPE"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_MARKING_CODE"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_MARKING_CODE_STATUS"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_MARKING_CODE_ONLINE_VALIDATION_RESULT"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_MARKING_PROCESSING_MODE"])
		fmt.Println(parametersIn["1212"])
		fmt.Println(parametersIn["1214"])
		fmt.Println(parametersIn["LIBFPTR_PARAM_MEASUREMENT_UNIT"])
		curPos := fmt.Sprint(len(registrationMap) + 1)
		registrationMap[curPos] = make(map[string]string)
		for kTemp, vTemp := range parametersIn {
			registrationMap[curPos][kTemp] = vTemp
		}
	case "payment":
		//LIBFPTR_PT_CASH           = 0
		//LIBFPTR_PT_ELECTRONICALLY = 1
		//LIBFPTR_PT_PREPAID        = 2
		//LIBFPTR_PT_CREDIT         = 3
		//LIBFPTR_PT_OTHER          = 4
		//LIBFPTR_PT_6              = 5
		//LIBFPTR_PT_7              = 6
		//LIBFPTR_PT_8              = 7
		//LIBFPTR_PT_9              = 8
		//LIBFPTR_PT_10             = 9
		var prevSummOfPayment float64
		var currSummOfPayment float64
		prevSummOfPayment = 0
		currSummOfPayment = 0
		prevSummOfPaymentStr := ""
		newSummOfPayStr := parametersIn["LIBFPTR_PARAM_PAYMENT_SUM"]
		fmt.Println(parametersIn["LIBFPTR_PARAM_PAYMENT_TYPE"]) //тип оплаты
		fmt.Println(parametersIn["LIBFPTR_PARAM_PAYMENT_SUM"])  //сумма типом оплаты
		typeOfPaymentTemp := parametersIn["LIBFPTR_PARAM_PAYMENT_TYPE"]
		if _, ok := paymentMap[typeOfPaymentTemp]; ok { //если один и тот же тип оплаты несколько раз
			prevSummOfPaymentStr := paymentMap[typeOfPaymentTemp]
			if summTemp, err := strconv.ParseFloat(prevSummOfPaymentStr, 64); err == nil {
				prevSummOfPayment = summTemp
			} else {
				logsmap[LOGERROR].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
					"zavNum", deviceMap["zavNum"], "оброботка команды", command, "ошибка (%v) парсинга уже имеющейся суммы %v оплаты (тип оплаты %v)", err, prevSummOfPaymentStr, typeOfPaymentTemp)
			}
		}
		if prevSummOfPaymentStr != "" {
			currSummStr := parametersIn["LIBFPTR_PARAM_PAYMENT_SUM"]
			if summTemp, err := strconv.ParseFloat(currSummStr, 64); err == nil {
				currSummOfPayment = summTemp
			} else {
				logsmap[LOGERROR].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
					"zavNum", deviceMap["zavNum"], "оброботка команды", command, "ошибка (%v) парсинга текщей суммы %v оплаты (тип оплаты %v)", err, currSummStr, typeOfPaymentTemp)
			}
			newSummOfPay := prevSummOfPayment + currSummOfPayment
			newSummOfPayStr = fmt.Sprint(newSummOfPay)
		}
		paymentMap[typeOfPaymentTemp] = newSummOfPayStr
	case "receipt_total":
		//запоминаем сумма чека
		fmt.Println(parametersIn["LIBFPTR_PARAM_SUM"]) //сумма чека
		*receiptTotal = parametersIn["LIBFPTR_PARAM_SUM"]
	case "close_receipt": //запоминаем чек
		//
	case "check_document_closed":
		//ничего не делаем
		fmt.Println(parametersOut["LIBFPTR_PARAM_DOCUMENT_CLOSED"])
		fmt.Println(parametersOut["LIBFPTR_PARAM_DOCUMENT_PRINTED"])
		//проверяем спсиок последних операций, если последние операции были оперции формирования чека, то
		//если LIBFPTR_PARAM_DOCUMENT_CLOSED = true, то отмечаем, что чек был закрыт
		//сохраняем чек на диск и очищаем буфер
	case "query_data":
		switch parametersIn["LIBFPTR_PARAM_DATA_TYPE"] {
		case "0", "16": //LIBFPTR_DT_STATUS, LIBFPTR_DT_SERIAL_NUMBER
			fmt.Println(parametersOut["LIBFPTR_PARAM_SERIAL_NUMBER"]) //заводской номер ККТ
			//запоминаем серийный номер для текущего deviceId команды
			deviceMap["zavNum"] = parametersOut["LIBFPTR_PARAM_SERIAL_NUMBER"]
		}
	case "fn_query_data":
		switch parametersIn["LIBFPTR_PARAM_FN_DATA_TYPE"] {
		case "2": //LIBFPTR_FNDT_FN_INFO
			//запоминаем текущего deviceId команды
			fmt.Println(parametersOut["LIBFPTR_PARAM_SERIAL_NUMBER"]) //ФН
			fmt.Println(parametersOut["LIBFPTR_PARAM_FN_VERSION"])    //"fn_v_1_1_2   "
			deviceMap["fn"] = parametersOut["LIBFPTR_PARAM_SERIAL_NUMBER"]
			deviceMap["ffdstr"] = parametersOut["LIBFPTR_PARAM_FN_VERSION"]
		case "7": //LIBFPTR_FNDT_FFD_VERSIONS
			//запоминаем текущего deviceId команды
			//LIBFPTR_FFD_UNKNOWN = 0,LIBFPTR_FFD_1_0 = 100,LIBFPTR_FFD_1_0_5 = 105,LIBFPTR_FFD_1_1 = 110,LIBFPTR_FFD_1_2 = 120
			fmt.Println(parametersOut["LIBFPTR_PARAM_FFD_VERSION"])
			deviceMap["ffdint"] = parametersOut["LIBFPTR_PARAM_FFD_VERSION"]
		case "9": //LIBFPTR_FNDT_REG_INFO
			//запоминаем текущего deviceId команды
			fmt.Println(parametersOut["1065"]) //ОСН
			fmt.Println(parametersOut["1209"]) //ffdVersion
			fmt.Println(parametersOut["1009"]) //organizationAddress
			fmt.Println(parametersOut["1018"]) //organizationVATIN
			fmt.Println(parametersOut["1048"]) //organizationName
			fmt.Println(parametersOut["1187"]) //paymentsAddress
			fmt.Println(parametersOut["1037"]) //регистрационный номер ККТ
			fmt.Println(parametersOut["1036"]) //заводской номер ККТ
			fmt.Println(parametersOut["1046"]) //ofdName
			deviceMap["osn"] = parametersOut["1065"]
			deviceMap["ffdtag"] = parametersOut["1209"]
			deviceMap["firmaddress"] = parametersOut["1009"]
			deviceMap["INN"] = parametersOut["1018"]
			deviceMap["name"] = parametersOut["1048"]
			deviceMap["place"] = parametersOut["1187"]
			deviceMap["regNum"] = parametersOut["1037"]
			deviceMap["zavNum"] = parametersOut["1036"]
			deviceMap["ofd"] = parametersOut["1046"]
		case "0": //LIBFPTR_FNDT_TAG_VALUE
			//ничего не делаем
			fmt.Println(parametersIn["LIBFPTR_PARAM_TAG_NUMBER"]) //номер тега
			fmt.Println(parametersOut["LIBFPTR_PARAM_TAG_VALUE"]) //его значение
		case "4", "5": //LIBFPTR_FNDT_LAST_RECEIPT, LIBFPTR_FNDT_LAST_DOCUMENT
			fmt.Println(parametersOut["LIBFPTR_PARAM_DOCUMENT_NUMBER"])
			//parametersIn["LIBFPTR_PARAM_RECEIPT_TYPE"]
			//parametersIn["LIBFPTR_PARAM_RECEIPT_SUM"]
			fmt.Println(parametersOut["LIBFPTR_PARAM_FISCAL_SIGN"])
			dateAndTimeDocum := parametersOut["LIBFPTR_PARAM_DATE_TIME"] //2023.11.22 09:16:00
			//запоминаем ФП, ФД, дату и время чека
			fmt.Println(dateAndTimeDocum) //Дата и время документа
			indSpace := strings.Index(dateAndTimeDocum, " ")
			dateDocum := strings.TrimSpace(dateAndTimeDocum[:indSpace])
			timeDoc := strings.TrimSpace(dateAndTimeDocum[indSpace:])
			fmt.Println(dateDocum) //дата документа
			fmt.Println(timeDoc)   //время документа
			//проверяем спсиок последних операций, если последние операции были оперции формирования чека, то
			if lastAssenatialActWasCloseReceipt(queueLastActs) {
				//сохраняем последний чек на диск и очищаем буфер
				//шапка чека
				reslHeaderReciept = make(map[string]string)
				reslHeaderReciept["deviceId"] = deviceId
				reslHeaderReciept["factoryNum"] = deviceMap["zavNum"]
				reslHeaderReciept["regNum"] = deviceMap["regNum"]
				reslHeaderReciept["fn"] = deviceMap["fn"]
				reslHeaderReciept["ffdstr"] = deviceMap["ffdstr"]
				reslHeaderReciept["ffdint"] = deviceMap["ffdstr"]
				reslHeaderReciept["ffdtag"] = deviceMap["ffdtag"]
				reslHeaderReciept["INN"] = deviceMap["INN"]
				reslHeaderReciept["name"] = deviceMap["name"]
				if openReceiptMap["1187"] == "" {
					reslHeaderReciept["place"] = deviceMap["place"]
				} else {
					reslHeaderReciept["place"] = openReceiptMap["1187"]
				}
				reslHeaderReciept["cashier"] = operatorLoginMap["1021"]
				reslHeaderReciept["FD"] = parametersOut["LIBFPTR_PARAM_DOCUMENT_NUMBER"]
				reslHeaderReciept["FP"] = parametersOut["LIBFPTR_PARAM_FISCAL_SIGN"]
				reslHeaderReciept["dateandtime"] = parametersOut["LIBFPTR_PARAM_DATE_TIME"]
				reslHeaderReciept["date"] = dateDocum
				reslHeaderReciept["time"] = timeDoc
				if openReceiptMap["1055"] == "" {
					reslHeaderReciept["OSN"] = deviceMap["osn"]
				} else {
					reslHeaderReciept["OSN"] = openReceiptMap["1055"]
				}
				reslHeaderReciept["typeReciept"] = openReceiptMap["LIBFPTR_PARAM_RECEIPT_TYPE"]
				reslHeaderReciept["typeofcorrection"] = openReceiptMap["1173"]
				reslHeaderReciept["datecorrection"] = openReceiptMap["1178"]
				reslHeaderReciept["documNumofCorrection"] = openReceiptMap["1174"]
				reslHeaderReciept["thereMarks"] = strconv.FormatBool(registrationMapHaveMarks(registrationMap))
				//LIBFPTR_PT_CASH           = 0
				//LIBFPTR_PT_ELECTRONICALLY = 1
				//LIBFPTR_PT_PREPAID        = 2
				//LIBFPTR_PT_CREDIT         = 3
				//LIBFPTR_PT_OTHER          = 4
				//LIBFPTR_PT_6              = 5
				//LIBFPTR_PT_7              = 6
				//LIBFPTR_PT_8              = 7
				//LIBFPTR_PT_9              = 8
				//LIBFPTR_PT_10             = 9
				if *receiptTotal != "" {
					reslHeaderReciept["summ"] = *receiptTotal
				} else {
					reslHeaderReciept["summ"] = getSummStrFromRegistrationMap(namelogfile, numLine, deviceId,
						deviceMap["zavNum"], command, registrationMap, parametersOut["LIBFPTR_PARAM_DOCUMENT_NUMBER"])
				}
				reslHeaderReciept["nal"] = paymentMap["0"]
				reslHeaderReciept["beznal"] = paymentMap["1"]
				reslHeaderReciept["avance"] = paymentMap["2"]
				reslHeaderReciept["credit"] = paymentMap["3"]
				reslHeaderReciept["predstav"] = paymentMap["4"]
				reslHeaderReciept["6pay"] = paymentMap["5"]
				reslHeaderReciept["7pay"] = paymentMap["6"]
				reslHeaderReciept["8pay"] = paymentMap["7"]
				reslHeaderReciept["9pay"] = paymentMap["8"]
				reslHeaderReciept["10pay"] = paymentMap["9"]
				//registration postioan
				reslRegistrReciept = make(map[string]map[string]string)
				for numpos, pos := range registrationMap {
					reslRegistrReciept[numpos] = make(map[string]string)
					reslRegistrReciept[numpos]["name"] = pos["LIBFPTR_PARAM_COMMODITY_NAME"]
					reslRegistrReciept[numpos]["price"] = pos["LIBFPTR_PARAM_PRICE"]
					reslRegistrReciept[numpos]["quantity"] = pos["LIBFPTR_PARAM_QUANTITY"]
					reslRegistrReciept[numpos]["mera"] = pos["LIBFPTR_PARAM_MEASUREMENT_UNIT"]
					reslRegistrReciept[numpos]["amount"] = pos["LIBFPTR_PARAM_POSITION_SUM"]
					reslRegistrReciept[numpos]["predm"] = pos["1212"]
					reslRegistrReciept[numpos]["sposob"] = pos["1214"]
					reslRegistrReciept[numpos]["stavka"] = pos["LIBFPTR_PARAM_TAX_TYPE"]

					reslRegistrReciept[numpos]["buyername"] = pos["1227"]
					reslRegistrReciept[numpos]["buyerinn"] = pos["1228"]

					reslRegistrReciept[numpos]["agent"] = pos["1222"]
					reslRegistrReciept[numpos]["addressagent"] = pos["1016"]
					reslRegistrReciept[numpos]["innagent"] = pos["1026"]
					reslRegistrReciept[numpos]["nameagent"] = pos["1044"]
					reslRegistrReciept[numpos]["telephagent"] = pos["1073"]
					reslRegistrReciept[numpos]["telephoperpriem"] = pos["1074"]
					reslRegistrReciept[numpos]["telephoperperev"] = pos["1075"]

					reslRegistrReciept[numpos]["innspl"] = pos["1171"]
					reslRegistrReciept[numpos]["namespl"] = pos["1225"]

					reslRegistrReciept[numpos]["makr"] = pos["LIBFPTR_PARAM_MARKING_CODE"]
					reslRegistrReciept[numpos]["statusofmark"] = pos["LIBFPTR_PARAM_MARKING_CODE_STATUS"]
					reslRegistrReciept[numpos]["validationofmark"] = pos["LIBFPTR_PARAM_MARKING_CODE_ONLINE_VALIDATION_RESULT"]
					reslRegistrReciept[numpos]["processingmodeofmark"] = pos["LIBFPTR_PARAM_MARKING_PROCESSING_MODE"]
				}
				//checkWrite()
				//positionsOfCheckWrite
			}
		}
	default:
		logsmap[LOGINFO_WITHSTD].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
			"zavNum", deviceMap["zavNum"], "команды", command, "не будет обработана")
	}
	return reslHeaderReciept, reslRegistrReciept, "", nil
}

func ReadAtolLogFile(atolLogFile string, baseReceiptHeader, baseReceiptPostions *csv.Writer) (string, error) {
	//var deviceIdSerialNumber map[string]string
	//var deviceIdRegNumber map[string]string
	//var deviceIdSerialFN map[string]string
	//var deviceIdParameters map[string]map[string]string
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

	QueueLastCommandsByDeviceId := make(map[string]*[LEN_QUEUE_BUFFER_LOGS_STRING]string)
	QueueLastRightCommandsByDeviceId := make(map[string]*[LEN_QUEUE_BUFFER_LOGS_STRING]string)

	var InParametersLastCommandByDeviceId map[string]map[string]string
	var OutParametersLastCommandByDeviceId map[string]map[string]string
	InParametersLastCommandByDeviceId = make(map[string]map[string]string)
	OutParametersLastCommandByDeviceId = make(map[string]map[string]string)

	var deviceMapByDeviceId map[string]map[string]string                  //deviceId, имя параметра кассы, значение
	var registrationMapByDeviceId map[string]map[string]map[string]string //deviceId, номер позиции, параметры позиции
	var paymentMapByDeviceId map[string]map[string]string                 //drviceId, тип оплаты, сумма
	var operatorLoginMapByDeviceId map[string]map[string]string           //deviceId, имя параметра, значение
	var openReceiptMapByDeviceId map[string]map[string]string             //deviceId, имя параметра, значение
	var receiptTotalMapByDeviceId map[string]*string                      //deviceId, сумма чека
	deviceMapByDeviceId = make(map[string]map[string]string)
	registrationMapByDeviceId = make(map[string]map[string]map[string]string)
	operatorLoginMapByDeviceId = make(map[string]map[string]string)
	openReceiptMapByDeviceId = make(map[string]map[string]string)
	paymentMapByDeviceId = make(map[string]map[string]string)
	receiptTotalMapByDeviceId = make(map[string]*string)

	//csv файл чеков
	//flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	/*flagsTempOpen := os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	file_checks, err := os.OpenFile(RESULTSDIR, flagsTempOpen, 0644)
	if err != nil {
		descrError := fmt.Sprintf("ошибка создания файла чеков %v", err)
		logsmap[LOGERROR].Println(descrError)
		return descrError, err
	}
	defer file_checks.Close()
	csv_checks := csv.NewWriter(file_checks)
	defer csv_checks.Flush()*/
	InParametersLastCommand := make(map[string]string)
	outParametersLastCommand := make(map[string]string)
	gettingOutParameters := false
	waitOutParameters := false
	currNumLine := 0
	currDeviceId := ""
	sTemp := strings.Replace(atolLogFile, "\\", "/", -1)
	shortFileNameLog := path.Base(sTemp)
	for scannerLogAtol.Scan() {
		currNumLine++
		line := scannerLogAtol.Bytes()
		if thisLineHasNoContent(shortFileNameLog, line, currNumLine) {
			continue
		}
		//                                                             62
		//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
		afterFiscPrint := strings.TrimSpace(string(line[62:]))
		//open_shift,check_document_closed,fn_query_data,open_shift,cancel_receipt,
		//begin_nonfiscal_document,print_text,operator_login,open_receipt,registration,
		//receipt_total,payment,close_receipt,query_data
		AddLineInQueue(&QueueLastStrings, string(line))

		logsmap[LOGINFO].Println("logfile", shortFileNameLog, "line", currNumLine, string(line))
		//если предыдущая строка была командой
		if QueueLastStrings[1] != "" {
			afterFiscPrintPrev := strings.TrimSpace(QueueLastStrings[1][62:])
			if isCommandLogLine(afterFiscPrintPrev) { //это строка команды
				//получаем девайс id команды
				//send header sign=[B65D9C62] deviceID=[39515DDE99BEB2C2D17738FEAF401B0B2C47CB6D] id=0176 type=[REQUEST] len=94
				currDeviceId = getDeviceIdofCommand(afterFiscPrint)
				if currDeviceId != "" {
					_, ok := QueueLastCommandsByDeviceId[currDeviceId]
					if !ok {
						QueueLastCommandsByDeviceId[currDeviceId] = new([LEN_QUEUE_BUFFER_LOGS_STRING]string)
						InParametersLastCommandByDeviceId[currDeviceId] = make(map[string]string)
					} else {
						//очищаем входные и выходные параметры для новой команды
						if _, ok := InParametersLastCommandByDeviceId[currDeviceId]; ok {
							clearParametersOfCommand(InParametersLastCommandByDeviceId[currDeviceId])
						}
						if _, ok := OutParametersLastCommandByDeviceId[currDeviceId]; ok {
							clearParametersOfCommand(OutParametersLastCommandByDeviceId[currDeviceId])
						}
					}
					AddLineInQueue(QueueLastCommandsByDeviceId[currDeviceId], QueueLastCommands[0])
					//запоминаем входные параметры команды
					//InParametersLastCommandByDeviceId[currDeviceId] = InParametersLastCommand
					for kTemp, vTemp := range InParametersLastCommand {
						InParametersLastCommandByDeviceId[currDeviceId][kTemp] = vTemp
					}
				}
			}
		}
		//обработка результата выполнения команды
		if isResultExecCommand(afterFiscPrint) { //это результат выполнения комманды
			if isCommandWasExecSuссess(afterFiscPrint) { //команда выполнена успешно
				//запоминаем успешно выполненную команду
				AddLineInQueue(&QueueLastRightCommands, QueueLastCommands[0])
				if currDeviceId != "" {
					if _, ok := QueueLastRightCommandsByDeviceId[currDeviceId]; !ok {
						QueueLastRightCommandsByDeviceId[currDeviceId] = new([LEN_QUEUE_BUFFER_LOGS_STRING]string)
					}
					AddLineInQueue(QueueLastRightCommandsByDeviceId[currDeviceId], QueueLastCommands[0])
				}
				if commandHasOutputParameters(QueueLastRightCommands[0]) { //команда имеет выходные парамтеры
					//ставим флаг ожидания выходных параметров
					waitOutParameters = true
				} else { //
					//запускаем обработку результата
					if currDeviceId != "" {
						if _, ok := registrationMapByDeviceId[currDeviceId]; !ok {
							registrationMapByDeviceId[currDeviceId] = make(map[string]map[string]string)
						}
						if _, ok := paymentMapByDeviceId[currDeviceId]; !ok {
							paymentMapByDeviceId[currDeviceId] = make(map[string]string)
						}
						if _, ok := operatorLoginMapByDeviceId[currDeviceId]; !ok {
							operatorLoginMapByDeviceId[currDeviceId] = make(map[string]string)
						}
						if _, ok := openReceiptMapByDeviceId[currDeviceId]; !ok {
							openReceiptMapByDeviceId[currDeviceId] = make(map[string]string)
						}
						if _, ok := receiptTotalMapByDeviceId[currDeviceId]; !ok {
							receiptTotalMapByDeviceId[currDeviceId] = new(string)
						}
						if _, ok := deviceMapByDeviceId[currDeviceId]; !ok {
							deviceMapByDeviceId[currDeviceId] = make(map[string]string)
						}
						_, _, errodDiscr, err := processingResultOfCommand(shortFileNameLog, currNumLine,
							currDeviceId, deviceMapByDeviceId[currDeviceId],
							QueueLastRightCommandsByDeviceId[currDeviceId][0],
							*QueueLastRightCommandsByDeviceId[currDeviceId], InParametersLastCommandByDeviceId[currDeviceId],
							OutParametersLastCommandByDeviceId[currDeviceId], registrationMapByDeviceId[currDeviceId],
							operatorLoginMapByDeviceId[currDeviceId], openReceiptMapByDeviceId[currDeviceId],
							paymentMapByDeviceId[currDeviceId], receiptTotalMapByDeviceId[currDeviceId])
						if err != nil {
							descrError := fmt.Sprintf("ошибка (%v) обработки комманды %v в лог файле %v атол в строке %v",
								errodDiscr, QueueLastRightCommandsByDeviceId[currDeviceId][0], shortFileNameLog, currNumLine)
							logsmap[LOGERROR].Println(descrError)
						}
					}
				}
			} else { //если команда выплонена не успешно
				//очищаем входные параметры
				clearParametersOfCommand(InParametersLastCommand)
				if currDeviceId != "" {
					clearParametersOfCommand(InParametersLastCommandByDeviceId[currDeviceId])
				}
			} //успешно или не успешно выполнена команда
		} //строка результата выполнения команды
		if isOutputFromKKTParameter(afterFiscPrint) && waitOutParameters { //если это выходной параметр и была упешно выполнена команда, для которой мы читаем выходные параметры
			//ставим флаг, что сейчас мы читаем выходные параметры ККТ
			gettingOutParameters = true
			//убираем флаг того, что мы ждём параметры ещё параметры
			waitOutParameters = false
		}
		if gettingOutParameters { //если идут строки выходных параметров последней команды
			if isOutputFromKKTParameter(afterFiscPrint) { //если это выходной параметр
				//запоминаем выходной параметр
				//< LIBFPTR_PARAM_DATE_TIME (65590) = 2023.11.22 08:31:25
				parametrNameAndValue := afterFiscPrint[2:]
				//LIBFPTR_PARAM_DATE_TIME (65590) = 2023.11.22 08:31:25
				indSpace := strings.Index(parametrNameAndValue, " ")
				parametrName := parametrNameAndValue[:indSpace]
				indEqual := strings.Index(parametrNameAndValue, "=")
				ValueOfParam := strings.TrimSpace(parametrNameAndValue[indEqual+1:])
				outParametersLastCommand[parametrName] = ValueOfParam
				if currDeviceId != "" {
					_, ok := OutParametersLastCommandByDeviceId[currDeviceId]
					if !ok {
						OutParametersLastCommandByDeviceId[currDeviceId] = make(map[string]string)
					}
					OutParametersLastCommandByDeviceId[currDeviceId][parametrName] = ValueOfParam
				}
			} else { //закончили читать выходные параметры
				//убираем флаг, что сейчас читаем выходные параметры
				gettingOutParameters = false
				//запускаем обработку результата
				if currDeviceId != "" {
					if _, ok := registrationMapByDeviceId[currDeviceId]; !ok {
						registrationMapByDeviceId[currDeviceId] = make(map[string]map[string]string)
					}
					if _, ok := paymentMapByDeviceId[currDeviceId]; !ok {
						paymentMapByDeviceId[currDeviceId] = make(map[string]string)
					}
					if _, ok := operatorLoginMapByDeviceId[currDeviceId]; !ok {
						operatorLoginMapByDeviceId[currDeviceId] = make(map[string]string)
					}
					if _, ok := openReceiptMapByDeviceId[currDeviceId]; !ok {
						openReceiptMapByDeviceId[currDeviceId] = make(map[string]string)
					}
					if _, ok := receiptTotalMapByDeviceId[currDeviceId]; !ok {
						receiptTotalMapByDeviceId[currDeviceId] = new(string)
					}
					if _, ok := deviceMapByDeviceId[currDeviceId]; !ok {
						deviceMapByDeviceId[currDeviceId] = make(map[string]string)
					}
					headerReceipt, postionsReceipt, errodDiscr, err := processingResultOfCommand(shortFileNameLog, currNumLine, currDeviceId,
						deviceMapByDeviceId[currDeviceId],
						QueueLastRightCommandsByDeviceId[currDeviceId][0],
						*QueueLastRightCommandsByDeviceId[currDeviceId],
						InParametersLastCommandByDeviceId[currDeviceId],
						OutParametersLastCommandByDeviceId[currDeviceId],
						registrationMapByDeviceId[currDeviceId],
						operatorLoginMapByDeviceId[currDeviceId],
						openReceiptMapByDeviceId[currDeviceId], paymentMapByDeviceId[currDeviceId],
						receiptTotalMapByDeviceId[currDeviceId])
					if err != nil {
						descrError := fmt.Sprintf("ошибка (%v) обработки комманды %v в лог файле %v атол в строке %v",
							errodDiscr, QueueLastRightCommandsByDeviceId[currDeviceId][0], shortFileNameLog, currNumLine)
						logsmap[LOGERROR].Println(descrError)
					} else {
						if len(headerReceipt) > 0 {
							writeRecordOfReceipt(shortFileNameLog, currNumLine, currDeviceId,
								deviceMapByDeviceId[currDeviceId]["zavNum"], baseReceiptHeader, headerReceipt)
							if len(postionsReceipt) > 0 {
								writeRecordsOfRegistartionReceipt(shortFileNameLog, currNumLine, currDeviceId,
									deviceMapByDeviceId[currDeviceId]["zavNum"],
									baseReceiptPostions, postionsReceipt, headerReceipt)
							} else {
								descrAlgorith := fmt.Sprintf("при обработке комманды %v (фомрировании чека) в лог файле %v атол в строке %v полчилось, что у чека нет позиций", QueueLastRightCommandsByDeviceId[currDeviceId][0], shortFileNameLog, currNumLine)
								logsmap[LOGERROR].Println(descrAlgorith)
							}
						}
					} //результат обработки команды
				} //если deviceId заполнено
			} //закончили читать выходные параметры
		} //если мы находимся в процессе чтения выходных параметров
		if isCommandLogLine(afterFiscPrint) { //это строка команды
			//очищаем предыдущие входные и выходные параметры
			clearParametersOfCommand(InParametersLastCommand)
			clearParametersOfCommand(outParametersLastCommand)
			currCommand := strings.TrimSpace(afterFiscPrint)
			currCommand, _ = strings.CutPrefix(currCommand, "libfptr_")
			currCommand, _ = strings.CutSuffix(currCommand, "()")
			currCommand = strings.TrimSpace(currCommand)
			AddLineInQueue(&QueueLastCommands, currCommand)
			//запоминаем входные параметры команды
			InParametersLastCommand = GetAllParametersCurrentCommand(QueueLastStrings, currNumLine)
		} //это строка команды
	} //for scannerLogAtol.Scan() {
	file_atol_log.Close()
	return "", nil
} //ReadAtolLogFile

func AddLineInQueue(queueLastStrings *[LEN_QUEUE_BUFFER_LOGS_STRING]string, s string) error {
	for i := LEN_QUEUE_BUFFER_LOGS_STRING - 1; i > 0; i-- {
		queueLastStrings[i] = queueLastStrings[i-1]
	}
	queueLastStrings[0] = s
	return nil
}

func clearParametersOfCommand(parametersCommand map[string]string) {
	for k := range parametersCommand {
		delete(parametersCommand, k)
	}
}

func GetAllParametersCurrentCommand(QueueStrings [LEN_QUEUE_BUFFER_LOGS_STRING]string, lineNum int) map[string]string {
	parametersCommand := make(map[string]string)
	for i := 1; i < LEN_QUEUE_BUFFER_LOGS_STRING; i++ {
		obrStr := strings.TrimSpace(QueueStrings[i])
		if obrStr == "" {
			continue
		}
		//logsmap[LOGINFO_WITHSTD].Println("logfile", namelogfile, "lineNum", lineNum, "---", obrStr)
		//fmt.Println(obrStr)
		afterFiscPrintObrStr := strings.TrimSpace(obrStr[62:])
		if isCommandLogLine(afterFiscPrintObrStr) {
			break
		}
		//> LIBFPTR_PARAM_TAX_SUM (65570) = 9810.7
		if afterFiscPrintObrStr[0:1] == ">" {
			//> LIBFPTR_PARAM_TAX_SUM (65570) = 9810.7
			parametrNameAndValue := afterFiscPrintObrStr[2:]
			//LIBFPTR_PARAM_TAX_SUM (65570) = 9810.7
			indSpace := strings.Index(parametrNameAndValue, " ")
			parametrName := parametrNameAndValue[:indSpace]
			indEqual := strings.Index(parametrNameAndValue, "=")
			ValueOfParam := strings.TrimSpace(parametrNameAndValue[indEqual+1:])
			parametersCommand[parametrName] = ValueOfParam
		} else {
			break //закончились параметры команды
		}
	} //перебор всех параметров команды
	return parametersCommand
} //GetAllParametersCurrentCommand

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
			return spisFiles, err
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
} //listDirByReadDir

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
	flagsTempOpen := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if clearLogs {
		flagsTempOpen = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}
	f, err := os.OpenFile(logFile, flagsTempOpen, 0644)
	multwr := io.MultiWriter(f)
	//if pref == LOG_PREFIX+"_INFO" {
	//	multwr = io.MultiWriter(f, os.Stdout)
	//}
	flagsLogs := log.LstdFlags
	if pref == LOG_PREFIX+"_ERROR" {
		multwr = io.MultiWriter(f, os.Stderr)
		flagsLogs = log.LstdFlags | log.Lshortfile
	}
	if err != nil {
		fmt.Println("Не удалось создать лог файл ", logFile, err)
		return nil, nil, err
	}
	loger := log.New(multwr, pref+" ", flagsLogs)
	return f, loger, nil
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
	if err != nil {
		descrError := fmt.Sprintf("Не удалось создать компоненту gzip для разархивирования "+
			"файла %v c ошибкой: %v", fileName, err)
		logsmap[LOGERROR].Println(descrError)
		return newFileName, descrError, err
	}
	uncompressedFile, err := os.Create(newFileName)
	if err != nil {
		descrError := fmt.Sprintf("Не удалось создать разархивированный лог файл %v атол c ошибкой: %v", fileName, err)
		logsmap[LOGERROR].Println(descrError)
		return newFileName, descrError, err
	}
	_, err = io.Copy(uncompressedFile, gzipReader)
	if err != nil {
		descrError := fmt.Sprintf("ошибка копирования буфера (разархивирования) файла %v c ошибкой: %v", fileName, err)
		logsmap[LOGERROR].Println(descrError)
		return newFileName, descrError, err
	}
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
	}
	return
}

// nextField1 nextField2
// recv data { "e" : { "c" : 0, "d" : "Ошибок нет" }, "p" : [ { "t" : 0, "v" : 105, "n" : 65627, "s" : 0 }, { "t" : 0, "v" : 110, "n" : 65628, "s" : 0 }, { "t" : 0, "v" : 105, "n" : 65629, "s" : 0 }, { "t" : 5, "v" : { "$date" : "1970-01-18T05:54:14.400Z" }, "n" : 65590, "s" : 0 }, { "t" : 0, "v" : 105, "n" : 65692, "s" : 0 }, { "t" : 0, "v" : 120, "n" : 65693, "s" : 0 }, { "t" : 0, "v" : 261, "n" : 65753, "s" : 0 }, { "t" : 0, "v" : 110, "n" : 65819, "s" : 0 } ], "f" : null }
func isResultExecCommand(logstring string) bool {
	nextField1 := logstring[:4]
	nextField2 := logstring[5:9]
	if (nextField1 == "recv") && (nextField2 == "data") {
		return true
	}
	return false
}

// это строка команды
func isCommandLogLine(logstring string) bool {
	return logstring[:7] == "libfptr"
}

func isCommandWasExecSuссess(logstring string) bool {
	return strings.Contains(logstring, "Ошибок нет")
}

// open_shift,check_document_closed,fn_query_data,cancel_receipt,
// begin_nonfiscal_document,print_text,operator_login,open_receipt,registration,
// receipt_total,payment,close_receipt,query_data
func commandHasOutputParameters(command string) bool {
	/*switch command {
		case "open_shift", "cancel_receipt", "begin_nonfiscal_document",
		"print_text", "operator_login", "open_receipt", "registration", "receipt_total",
		"payment", "close_receipt": return false
	}*/
	switch command {
	case "check_document_closed", "fn_query_data", "query_data", "read_device_setting", "model_flags":
		return true
	}
	return false
}

func isOutputFromKKTParameter(logstring string) bool {
	return logstring[0:1] == "<"
}

func thisLineHasNoContent(namelogfile string, line []byte, numOfLine int) bool {
	//                                                             62
	//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
	if len(line) < 62 {
		logsmap[LOGSKIP_LINES].Println("logfile", namelogfile, "маленький размер", "line", numOfLine, string(line))
		return true
	}
	//                                               47
	//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
	srCurr := line[47 : 47+len("[FiscalPrinter]")]
	if !bytes.Equal(srCurr, []byte("[FiscalPrinter]")) {
		logsmap[LOGSKIP_LINES].Println("logfile", namelogfile, "нет [FP]", "line", numOfLine, "slices", string(srCurr), string(line))
		return true
	}
	//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
	//2023.11.22 13:55:59.097       T:0000DAE0 INFO  [FiscalPrinter] > LIBFPTR_PARAM_COMMODITY_NAME (65631) = "Керамогранит 600*1200 CF101 MR ГРАНИТЕ САНДРА белый (45.36/2.16)"
	//дата,время,что-то еще,тип сообщения,тип оборудования,напровление (>на кассу,<из кассы),команда, id команды, знак равно, значение команды
	//2023.11.22 14:01:21.190       T:0000DAE0 INFO  [FiscalPrinter] < LIBFPTR_PARAM_DEVICE_FFD_VERSION (65627) = 105
	afterFiscPrint := strings.TrimSpace(string(line[62:]))
	return len(afterFiscPrint) < 9
}

func getDeviceIdofCommand(shortLineOfLog string) string {
	//send header sign=[B65D9C62] deviceID=[39515DDE99BEB2C2D17738FEAF401B0B2C47CB6D] id=0176 type=[REQUEST] len=94
	indDeviceId := strings.Index(shortLineOfLog, "deviceID")
	if indDeviceId == -1 {
		return ""
	}
	rightObrStr := shortLineOfLog[indDeviceId:]
	indEqual := strings.Index(rightObrStr, "=")
	if indEqual == -1 {
		return ""
	}
	rightObrStr = rightObrStr[indEqual+2:]
	indZakrSc := strings.Index(rightObrStr, "]")
	if indZakrSc == -1 {
		return ""
	}
	deviceIdStr := rightObrStr[:indZakrSc]
	return deviceIdStr
}

func writeRecordOfReceipt(namelogfile string, numLine int, deviceId string, zavNum string, baseReceiptHeader *csv.Writer,
	checkHeader map[string]string) (string, error) {
	recordCheck := make([]string, len(checkHeader))
	indField := 0
	for _, v := range checkHeader {
		recordCheck[indField] = v
		indField++
	}
	err := baseReceiptHeader.Write(recordCheck)
	if err != nil {
		decrError := fmt.Sprintf("ошибка записи в базу информации о чеке. Ошибка: %v", err)
		logsmap[LOGERROR].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
			"zavNum", zavNum, decrError)
		return decrError, err
	}
	return "", nil
}

func writeRecordsOfRegistartionReceipt(namelogfile string, numLine int, deviceId string, zavNum string,
	baseReceiptPostions *csv.Writer, postionsReceipt map[string]map[string]string, checkHeader map[string]string) (string, error) {
	var recordPosition []string

	for numPos, pos := range postionsReceipt {
		if len(recordPosition) == 0 {
			recordPosition = make([]string, len(pos)+5)
		} else {
			for i := 0; i < len(recordPosition); i++ {
				recordPosition[i] = ""
			}
		}
		recordPosition[0] = checkHeader["INN"]        //ИНН
		recordPosition[1] = checkHeader["factoryNum"] //заводской номер ККТ
		recordPosition[2] = checkHeader["fn"]         //ФН
		recordPosition[3] = checkHeader["FD"]         //ФД
		recordPosition[4] = checkHeader["summ"]       //сумма чека
		indFiled := 5
		for _, v := range pos {
			recordPosition[indFiled] = v
			indFiled++
		}
		err := baseReceiptPostions.Write(recordPosition)
		if err != nil {
			decrError := fmt.Sprintf("ошибка записи в базу позиции %v чека. Ошибка: %v", numPos, err)
			logsmap[LOGERROR].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
				"zavNum", zavNum, decrError)
			return decrError, err
		}
	}
	return "", nil
}

func registrationMapHaveMarks(registrationMap map[string]map[string]string) bool {
	res := false
	for _, pos := range registrationMap {
		if len(pos["LIBFPTR_PARAM_MARKING_CODE"]) > 10 {
			res = true
			break
		}
	}
	return res
}

func getSummStrFromRegistrationMap(namelogfile string, numLine int, deviceId string,
	zavNum string, command string, registrationMap map[string]map[string]string, checkNumber string) string {
	var currSumm float64
	var totalSumm float64
	res := "0"
	totalSumm = 0
	for numPos, pos := range registrationMap {
		currSummStr := pos["LIBFPTR_PARAM_POSITION_SUM"]
		currSumm = 0
		if summTemp, err := strconv.ParseFloat(currSummStr, 64); err == nil {
			currSumm = summTemp
		} else {
			descrErr := fmt.Sprintf("ошибка (%v) парсинга суммы %v в %v позиции чека №%v",
				err, currSummStr, numPos, checkNumber)
			logsmap[LOGERROR].Println("logfile", namelogfile, "line", numLine, "deviceId", deviceId,
				"zavNum", zavNum, "оброботка команды", command, descrErr)
		}
		totalSumm = totalSumm + currSumm
	}
	res = fmt.Sprint(totalSumm)
	return res
}
