package main

import (
	"bufio"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
)

type Product struct {
	ID         uint32
	CategoryID uint64
	Brand      [100]byte
	Price      float32
}

type Category struct {
	ID   uint32
	Name [100]byte
}

type Action uint8

const (
	View           Action = 1 << iota // 0001
	Cart                              // 0010
	RemoveFromCart                    // 0100
	Purchase                          // 1000
)

const (
	EVENT_TIME    = iota // 0
	EVENT_TYPE           // 1
	PRODUCT_ID           // 2
	CATEGORY_ID          // 3
	CATEGORY_CODE        // 4
	BRAND                // 5
	PRICE                // 6
	USER_ID              // 7
	USER_SESSION         // 8
)

type Event struct {
	ID          uint32
	UserSession [50]byte
	UserID      uint32
	ProductID   uint32
	EventAction Action
	EventTime   [100]byte
}

type IndexEntry struct {
	ID     uint32
	Offset int64
}

func getActionFromName(actionName string) Action {
	switch actionName {
	case "cart":
		return Cart
	case "remove_from_cart":
		return RemoveFromCart
	case "purchase":
		return Purchase
	default:
		return View
	}
}
func getActionName(action Action) string {
	switch action {
	case Cart:
		return "cart"
	case RemoveFromCart:
		return "remove_from_cart"
	case Purchase:
		return "purchase"
	default:
		return "view"
	}
}

func StringTo50ByteArray(str string) [50]byte {
	var arr [50]byte
	copy(arr[:], str)
	return arr
}
func StringToByteArray(str string) [100]byte {
	var arr [100]byte
	copy(arr[:], str)
	return arr
}

func AppendDataToFile[T any](file *os.File, data T) (int64, error) {
	// Busca o offset atual
	offset, err := file.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, err
	}

	// Escreve o registro no arquivo de dados
	err = binary.Write(file, binary.LittleEndian, data)
	if err != nil {
		return 0, err
	}

	// Retorna o offsert do registro gravada
	return offset, nil
}
func AppendIndexToFile(file *os.File, id uint32, offset int64) error {
	// Cria a entrada do índice
	entry := IndexEntry{
		ID:     id,
		Offset: offset,
	}

	// Escreve a entrada no arquivo
	return binary.Write(file, binary.LittleEndian, entry)
}

func CreateFile(fileName string, errorMessage string) *os.File {
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf(errorMessage)
	}
	defer file.Close()
	return file
}

func Append[T any](dataFile *os.File, indexFile *os.File, data T, id uint32) error {
	offset, err := AppendDataToFile(dataFile, data)
	if err != nil {
		log.Fatalf("Nao foi possivel salvar registro no arquivo %s: %v", dataFile.Name(), err)
	}

	return AppendIndexToFile(indexFile, id, offset)
}

func main() {

	file, err := os.Open("2019-Oct.csv")
	if err != nil {
		log.Fatalf("Erro ao abrir arquivo")
	}
	defer file.Close()

	productsDataFile := CreateFile("products_data.bin", "Erro ao criar arquivo de dados produtos")
	productsIndexFile := CreateFile("products_index.bin", "Erro ao criar arquivo de index produtos")

	categorysDataFile := CreateFile("categorys_data.bin", "Erro ao criar arquivo de dados categorias")
	categorysIndexFile := CreateFile("categorys_index.bin", "Erro ao criar arquivo de index categorias")

	eventsDataFile := CreateFile("events_data.bin", "Erro ao criar arquivo de dados eventos")
	eventsIndexFile := CreateFile("events_index.bin", "Erro ao criar arquivo de index eventos")

	reader := bufio.NewReader(file)
	csvReader := csv.NewReader(reader)

	fmt.Printf("Readers criados")

	_, err = csvReader.Read()
	if err != nil {
		log.Fatalf("Erro ao ler header")
	}
	productId := 0
	categoryId := 0
	eventId := 0

	addedProducts := make(map[uint32]int)
	addedCategorys := make(map[uint64]int)
	addedEvents := make(map[string]int)
	for {
		column, err := csvReader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatalf("Erro ao ler o arquivo")
		}

		productPrice, err := strconv.ParseFloat(column[PRICE], 32)
		if err != nil {
			log.Fatalf("Erro ao converter %s para float: %v", column[1], err)
		}

		//Verifica se o produto já foi adicionado para evitar repetições
		csvProductId, _ := strconv.Atoi(column[PRODUCT_ID])
		_, exists := addedProducts[uint32(csvProductId)]
		if !exists {
			product := Product{
				ID:         uint32(productId),
				CategoryID: uint64(categoryId),
				Brand:      StringToByteArray(column[BRAND]),
				Price:      float32(productPrice),
			}
			Append(productsDataFile, productsIndexFile, product, product.ID)
			productId++
			// Adiciona o produto no map de já adicionados
			addedProducts[uint32(csvProductId)] = productId
		}

		//Verifica se a categoria já foi adicionada para evitar repetições
		csvCategoryId, _ := strconv.Atoi(column[CATEGORY_ID])
		_, exists = addedCategorys[uint64(csvCategoryId)]
		if !exists {
			category := Category{
				ID:   uint32(categoryId),
				Name: StringToByteArray(column[CATEGORY_CODE]),
			}
			Append(categorysDataFile, categorysIndexFile, category, category.ID)
			categoryId++
			// Adiciona a categoria no map de já adicionados
			addedCategorys[uint64(csvCategoryId)] = categoryId
		}

		//Verifica se a sessão já foi adicionada para evitar repetições
		strUserSession := column[USER_SESSION]
		_, exists = addedEvents[strUserSession]
		if !exists {
			userSession := StringTo50ByteArray(strUserSession)
			userId, _ := strconv.Atoi(column[USER_ID])
			eventAction := getActionFromName(column[EVENT_TYPE])
			eventTime := StringToByteArray(column[EVENT_TIME])
			event := Event{
				ID:          uint32(eventId),
				UserSession: userSession,
				UserID:      uint32(userId),
				EventAction: eventAction,
				EventTime:   eventTime,
			}
			Append(eventsDataFile, eventsIndexFile, event, event.ID)
			eventId++
			addedEvents[strUserSession] = eventId
		}
	}
}
