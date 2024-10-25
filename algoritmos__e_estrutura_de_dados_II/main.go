package main

import (
	"bufio"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

type Product struct {
	ID         uint32
	CategoryID uint32
	Brand      [100]byte
	Price      float32
	Active     bool
}

type ProductMetrics struct {
	ProductID           uint32
	ProductDataLocation int64
	TotalPurchase       uint64
}

type Category struct {
	ID   uint32
	Name [100]byte
}

type Action uint8

const (
	VIEW             Action = 1 << iota // 0001
	CART                                // 0010
	REMOVE_FROM_CART                    // 0100
	PURCHASE                            // 1000
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
const (
	PRODUCT_DATA_FILE           = "products_data.bin"
	PRODUCT_INDEX_FILE          = "products_index.bin"
	MOST_EXPENSIVE_PRODUCT_FILE = "most_expensive_product.bin"

	CATEGORY_DATA_FILE  = "categorys_data.bin"
	CATEGORY_INDEX_FILE = "categorys_index.bin"

	EVENT_DATA_FILE     = "events_data.bin"
	EVENT_INDEX_FILE    = "events_index.bin"
	ACTION_METRICS_FILE = "action_metrics.bin"
)

type Event struct {
	ID          uint32
	UserSession [50]byte
	UserID      uint32
	ProductID   uint32
	EventAction Action
	EventTime   [100]byte
}

type ActionMetrics struct {
	Action             Action
	NumberOfOcurrences uint32
}

const (
	ACTION_KEY_SIZE   = 4
	ACTION_VALUE_SIZE = 32
)

type IndexEntry struct {
	ID     uint32
	Offset int64
}

func CreateOrOpenFile(filename string) *os.File {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return file
}
func getActionFromName(actionName string) Action {
	switch actionName {
	case "cart":
		return CART
	case "remove_from_cart":
		return REMOVE_FROM_CART
	case "purchase":
		return PURCHASE
	default:
		return VIEW
	}
}
func getActionName(action Action) string {
	switch action {
	case CART:
		return "cart"
	case REMOVE_FROM_CART:
		return "remove_from_cart"
	case PURCHASE:
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

func AppendDataToFile[T any](filename string, data T) (int64, error) {

	dataFile := CreateOrOpenFile(filename)
	defer dataFile.Close()

	// Busca o offset atual
	offset, err := dataFile.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	// Escreve o registro no arquivo de dados
	err = binary.Write(dataFile, binary.LittleEndian, data)
	if err != nil {
		fmt.Printf("Erro ao escrever no arquivo de dados: %v\n", err)
		return 0, err
	}

	// Garante que o buffer de escrita é enviado para o disco
	err = dataFile.Sync()
	if err != nil {
		return 0, err
	}

	// Retorna o offsert do registro gravada
	return offset, nil
}
func AppendIndexToFile(filename string, id uint32, offset int64) error {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	_, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Cria a entrada do índice
	entry := IndexEntry{
		ID:     id,
		Offset: offset,
	}

	// Escreve a entrada no arquivo
	return binary.Write(file, binary.LittleEndian, entry)
}

func Append[T any](dataFilename string, indexFilename string, data T, id uint32) error {
	offset, err := AppendDataToFile(dataFilename, data)
	if err != nil {
		log.Fatalf("Nao foi possivel salvar registro no arquivo %s: %v", dataFilename, err)
	}
	return AppendIndexToFile(indexFilename, id, offset)
}

func StoreActionMetrics(filename string, action Action) error {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	var storedMetrics ActionMetrics
	for {
		err := binary.Read(file, binary.LittleEndian, &storedMetrics)
		if err != nil {
			break
		}

		if storedMetrics.Action == action {
			storedMetrics.NumberOfOcurrences++
			file.Seek(-int64(binary.Size(storedMetrics)), os.SEEK_CUR)
			err = binary.Write(file, binary.LittleEndian, storedMetrics)
			if err != nil {
				return err
			}
			return nil
		}
	}

	newMetric := ActionMetrics{
		Action:             action,
		NumberOfOcurrences: 1,
	}
	err := binary.Write(file, binary.LittleEndian, &newMetric)
	if err != nil {
		log.Fatalf("Erro ao gravar métrica no map: %v", err)
	}
	return nil
}
func SearchActionMetrics(filename string, action Action) (ActionMetrics, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ActionMetrics{}, err
	}
	defer file.Close()

	var storedMetrics ActionMetrics
	for {
		err := binary.Read(file, binary.LittleEndian, &storedMetrics)
		if err != nil {
			break
		}

		if storedMetrics.Action == action {
			return storedMetrics, nil
		}
	}

	return ActionMetrics{
		Action:             action,
		NumberOfOcurrences: 0,
	}, nil
}

func ReadFromDataFile[T any](filename string, offset int64) T {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	var data T

	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		log.Fatalf("Erro ao posicionar ponteiro para o offset arquivo de dados: %v", err)
	}

	err = binary.Read(file, binary.LittleEndian, &data)
	if err != nil {
		log.Fatalf("Erro ao ler do arquivo de dados: %v", err)
	}
	return data
}
func BinarySearchOnDisk(primaryIndexFilename string, targetID uint32) (int64, bool) {

	primaryIndexFile := CreateOrOpenFile(primaryIndexFilename)
	defer primaryIndexFile.Close()

	fileInfo, err := primaryIndexFile.Stat()
	if err != nil {
		fmt.Printf("Erro ao consultar informacoes do arquivo: %v\n", err)
		return 0, false
	}

	recordSize := int64(binary.Size(IndexEntry{}))
	left := int64(0)
	right := fileInfo.Size()/recordSize - 1

	fmt.Printf("Record size: %v | File size: %v\n", recordSize, fileInfo.Size())
	fmt.Printf("Left: %d | Right: %d\n", left, right)

	for left <= right {
		mid := (left + right) / 2

		_, err = primaryIndexFile.Seek(mid*recordSize, io.SeekStart)
		if err != nil {
			log.Fatalf("Erro ao posicionar ponteiro para binary search: %v", err)
		}

		var record IndexEntry
		err = binary.Read(primaryIndexFile, binary.LittleEndian, &record)
		if err != nil {
			log.Fatalf("Erro ao ler arquivo para binary search: %v", err)
		}

		fmt.Printf("Mid value: %d | ID atual: %d | ID procurado: %d\n", mid, record.ID, targetID)
		if record.ID == targetID {
			fmt.Printf("ID encontrado\n")
			return record.Offset, true
		} else if record.ID < targetID {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return 0, false
}
func SearchMostExpensiveProduct(secondaryIndexFilename string) (Product, error) {
	secondaryIndexFile := CreateOrOpenFile(secondaryIndexFilename)
	defer secondaryIndexFile.Close()

	var mostExpensiveProduct Product
	err := binary.Read(secondaryIndexFile, binary.LittleEndian, &mostExpensiveProduct)
	if err != nil {
		log.Fatalf("Erro ao buscar produto mais caro")
		return Product{}, err
	}
	return mostExpensiveProduct, err
}
func RemoveProduct(dataFilename string, primaryIndexFilename string, secondaryIndexFilename string, id uint32) error {

	offset, found := BinarySearchOnDisk(primaryIndexFilename, id)
	if !found {
		return fmt.Errorf("Produto com ID %d não encontrado", id)
	}

	dataFile := CreateOrOpenFile(dataFilename)
	defer dataFile.Close()
	product := ReadFromDataFile[Product](dataFilename, offset)
	if product.Active {
		product.Active = false
		dataFile.Seek(offset, os.SEEK_SET)
		binary.Write(dataFile, binary.LittleEndian, &product)

		secondaryIndexFile := CreateOrOpenFile(secondaryIndexFilename)
		defer secondaryIndexFile.Close()
		mostExpensiveProduct, err := SearchMostExpensiveProduct(secondaryIndexFilename)
		if err != nil {
			return err
		}
		if product.ID == mostExpensiveProduct.ID {
			RecalculateMostExpensiveProduct(dataFilename, secondaryIndexFile)
		}
	}
	return nil
}
func RecalculateMostExpensiveProduct(productFilename string, secondaryIndexFile *os.File) {
	productFile := CreateOrOpenFile(productFilename)
	defer productFile.Close()

	var product Product
	var mostExpensiveProduct Product

	for {
		err := binary.Read(productFile, binary.LittleEndian, &product)
		if err != nil {
			break //EOF
		}
		if product.Active && product.Price > mostExpensiveProduct.Price {
			mostExpensiveProduct = product
		}
	}

	err := binary.Write(secondaryIndexFile, binary.LittleEndian, mostExpensiveProduct)
	if err != nil {
		log.Fatalf("Nao foi possivel atualizar o produto mais caro")
	}
	fmt.Println("Produto mais caro atualizado com sucesso")
}
func UpdateMostExpensiveProductIndex(secondaryIndexFilename string, product Product) error {
	secondaryIndexFile := CreateOrOpenFile(secondaryIndexFilename)
	defer secondaryIndexFile.Close()

	if !product.Active {
		return nil
	}

	var mostExpensiveProduct Product
	err := binary.Read(secondaryIndexFile, binary.LittleEndian, &mostExpensiveProduct)
	if err == nil {
		fmt.Printf("Produto atual: %.2f\n", product.Price)
		fmt.Printf("Produto mais caro: %.2f\n", mostExpensiveProduct.Price)
		if product.Price > mostExpensiveProduct.Price {
			secondaryIndexFile.Seek(io.SeekStart, io.SeekStart)
			err = binary.Write(secondaryIndexFile, binary.LittleEndian, product)
			if err != nil {
				return err
			}
		}
	} else {
		secondaryIndexFile.Seek(io.SeekStart, io.SeekStart)
		err = binary.Write(secondaryIndexFile, binary.LittleEndian, product)
		fmt.Print(secondaryIndexFile.Stat())
		if err != nil {
			fmt.Print(err)
			return err
		}
	}
	return nil
}

// Função genérica para retornar o tamanho de uma struct
// o go não permite consultar binary.Sizeof() de um tipo
// não concreto
func SizeOf[T any](value T) (int, error) {
	return binary.Size(value), nil
}
func RemoveProductFromDataFile[T any](dataFilename string, tempFilename string, offsetToRemove int64, dataType T) error {
	dataFile := CreateOrOpenFile(dataFilename)
	defer dataFile.Close()

	// Arquivo temporário para reorganizar os dados
	tempDataFile := CreateOrOpenFile(tempFilename)
	defer tempDataFile.Close()

	// Tamanho do registro de produto
	recordSize, err := SizeOf(dataType)
	if err != nil {
		fmt.Print(err)
	}

	// Percorre o arquivo atual
	currentOffset := int64(0)
	for {
		var product T

		err = binary.Read(dataFile, binary.LittleEndian, &product)
		if err == io.EOF {
			break // Fim do arquivo
		} else if err != nil {
			return err
		}

		// Será removido apenas o registro com offset igual ao procurado, o restante será copiado para o arquivo temporário
		if currentOffset != offsetToRemove {
			err = binary.Write(tempDataFile, binary.LittleEndian, product)
			if err != nil {
				return err
			}
		}
		currentOffset += int64(recordSize)
	}

	tempDataFile.Close()
	dataFile.Close()

	err = os.Remove(dataFilename)
	if err != nil {
		log.Fatalf("Falha ao remover arquivo: %v\n", err)
	}
	err = os.Rename(tempFilename, dataFilename)
	if err != nil {
		return err
	}
	return nil
}

func RemoveFromIndexFile(indexFilename string, idToRemove uint32) error {
	indexFile := CreateOrOpenFile(indexFilename)
	defer indexFile.Close()

	tempIndexFile := CreateOrOpenFile("temp_index.bin")
	defer tempIndexFile.Close()

	for {
		var indexEntry IndexEntry

		err := binary.Read(indexFile, binary.LittleEndian, &indexEntry)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if indexEntry.ID != idToRemove {
			err = binary.Write(tempIndexFile, binary.LittleEndian, indexEntry)
			if err != nil {
				return err
			}
		}
	}

	tempIndexFile.Close()
	indexFile.Close()
	err := os.Remove(indexFilename)
	if err != nil {
		log.Fatalf("Falha ao remover arquivo: %v\n", err)
	}
	err = os.Rename("temp_index.bin", indexFilename)
	if err != nil {
		return err
	}
	return nil
}

func RemoveByID[T any](indexFilename string, dataFilename string, tempFilename string, itemID uint32, dataType T) error {
	indexFile := CreateOrOpenFile(indexFilename)
	defer indexFile.Close()

	offset, found := BinarySearchOnDisk(indexFilename, itemID)
	if !found {
		return fmt.Errorf("Arquivo não encontrado\n")
	}
	err := RemoveProductFromDataFile(dataFilename, tempFilename, offset, dataType)
	if err != nil {
		log.Fatalf("Não foi possível remover registro do arquivo de dados: %v\n", err)
	}

	err = RemoveFromIndexFile(indexFilename, itemID)
	if err != nil {
		log.Fatalf("Não foi possível remover registro do arquivo de índices: %v\n", err)
	}

	return nil
}
func PrintAllProducts(filename string) {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	for {
		var product Product
		err := binary.Read(file, binary.LittleEndian, &product)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Não foi possível ler o arquivo: %v", err)
		}

		if product.Active {
			fmt.Printf(
				"{ID: %d, CategoryID: %d, Brand: %s, Price: %.2f}\n",
				product.ID,
				product.CategoryID,
				product.Brand,
				product.Price,
			)
		}

	}
}
func PrintAllCategorys(filename string) {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	for {
		var category Category
		err := binary.Read(file, binary.LittleEndian, &category)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Não foi possível ler o arquivo: %v", err)
		}

		fmt.Printf("{ID: %d, Name: %s}\n", category.ID, category.Name)
	}
}
func PrintAllEvents(filename string) {
	file := CreateOrOpenFile(filename)
	defer file.Close()

	for {
		var event Event
		err := binary.Read(file, binary.LittleEndian, &event)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Não foi possível ler o arquivo: %v", err)
		}

		fmt.Printf("{ID: %d, UserSession: %s, UserID: %d, ProductID: %d, EventAction: %s, EventTime: %s}\n",
			event.ID,
			event.UserSession,
			event.UserID,
			event.ProductID,
			getActionName(event.EventAction),
			event.EventTime,
		)

	}
}

func ReadLastProduct(dataFilename string) *Product {
	dataFile := CreateOrOpenFile(dataFilename)
	defer dataFile.Close()

	fileInfo, _ := dataFile.Stat()
	if fileInfo.Size() == 0 {
		return nil
	}

	recordSize := binary.Size(Product{})

	_, err := dataFile.Seek(-int64(recordSize), io.SeekEnd)
	if err != nil {
		log.Fatalf("Não foi possível olhar o último registro: %v\n", err)
	}

	var lastProduct Product
	err = binary.Read(dataFile, binary.LittleEndian, &lastProduct)
	if err != nil {
		log.Fatalf("Não foi possível ler o último registro: %v\n", err)
	}

	return &lastProduct
}
func ReadLastCategory(dataFilename string) *Category {
	dataFile := CreateOrOpenFile(dataFilename)
	defer dataFile.Close()

	fileInfo, _ := dataFile.Stat()
	if fileInfo.Size() == 0 {
		return nil
	}

	recordSize := binary.Size(Category{})

	_, err := dataFile.Seek(-int64(recordSize), io.SeekEnd)
	if err != nil {
		log.Fatalf("Não foi possível olhar o último registro: %v\n", err)
	}

	var lastCategory Category
	err = binary.Read(dataFile, binary.LittleEndian, &lastCategory)
	if err != nil {
		log.Fatalf("Não foi possível ler o último registro: %v\n", err)
	}

	return &lastCategory
}
func ReadLastEvent(dataFilename string) *Event {
	dataFile := CreateOrOpenFile(dataFilename)
	defer dataFile.Close()

	fileInfo, _ := dataFile.Stat()
	if fileInfo.Size() == 0 {
		return nil
	}

	recordSize := binary.Size(Event{})

	_, err := dataFile.Seek(-int64(recordSize), io.SeekEnd)
	if err != nil {
		log.Fatalf("Não foi possível olhar o último registro: %v\n", err)
	}

	var lastEvent Event
	err = binary.Read(dataFile, binary.LittleEndian, &lastEvent)
	if err != nil {
		log.Fatalf("Não foi possível ler o último registro: %v\n", err)
	}

	return &lastEvent
}

func BuildCategory(column []string) Category {
	var nextID uint32
	lastCategory := ReadLastCategory(CATEGORY_DATA_FILE)
	if lastCategory == nil {
		nextID = 0
	} else {
		nextID = lastCategory.ID + 1
	}
	category := Category{
		ID:   nextID,
		Name: StringToByteArray(column[CATEGORY_CODE]),
	}
	return category
}
func BuildProduct(column []string, productCategory Category) Product {
	var nextID uint32
	lastProduct := ReadLastProduct(PRODUCT_DATA_FILE)
	if lastProduct == nil {
		nextID = 0
	} else {
		nextID = lastProduct.ID + 1
	}
	productPrice, _ := strconv.ParseFloat(column[PRICE], 32)
	product := Product{
		ID:         uint32(nextID),
		CategoryID: productCategory.ID,
		Brand:      StringToByteArray(column[BRAND]),
		Price:      float32(productPrice),
		Active:     true,
	}
	return product
}
func BuildEvent(column []string) Event {
	var nextID uint32
	lastEvent := ReadLastEvent(EVENT_DATA_FILE)
	if lastEvent == nil {
		nextID = 0
	} else {
		nextID = lastEvent.ID + 1
	}
	userId, _ := strconv.Atoi(column[USER_ID])
	event := Event{
		ID:          nextID,
		UserSession: StringTo50ByteArray(column[USER_SESSION]),
		UserID:      uint32(userId),
		EventAction: getActionFromName(column[EVENT_TYPE]),
		EventTime:   StringToByteArray(column[EVENT_TIME]),
	}
	return event
}
func AddProduct(product Product) {
	Append(PRODUCT_DATA_FILE, PRODUCT_INDEX_FILE, product, product.ID)
	fmt.Printf("Adicionado produto de ID %d\n", product.ID)
	fmt.Printf("{ID: %d, CategoryID: %d, Brand: %s, Price: %.2f, Active: %t}\n", product.ID, product.CategoryID, product.Brand, product.Price, product.Active)
	UpdateMostExpensiveProductIndex(MOST_EXPENSIVE_PRODUCT_FILE, product)
}
func AddEvent(event Event) {
	Append(EVENT_DATA_FILE, EVENT_INDEX_FILE, event, event.ID)
	StoreActionMetrics(ACTION_METRICS_FILE, event.EventAction)
}
func ImportarCSV(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Erro ao abrir arquivo")
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	csvReader := csv.NewReader(reader)

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
			log.Fatalf("Erro ao ler o arquivo: %v", err)
		}
		//Verifica se a categoria já foi adicionada para evitar repetições
		csvCategoryId, _ := strconv.Atoi(column[CATEGORY_ID])
		_, exists := addedCategorys[uint64(csvCategoryId)]
		var category Category
		if !exists {
			category = BuildCategory(column)
			Append(CATEGORY_DATA_FILE, CATEGORY_INDEX_FILE, category, category.ID)
			// Adiciona a categoria no map de já adicionados
			addedCategorys[uint64(csvCategoryId)] = categoryId
		}

		//Verifica se o produto já foi adicionado para evitar repetições
		csvProductId, _ := strconv.Atoi(column[PRODUCT_ID])
		_, exists = addedProducts[uint32(csvProductId)]
		if !exists {
			product := BuildProduct(column, category)
			AddProduct(product)
			// Adiciona o produto no map de já adicionados
			addedProducts[uint32(csvProductId)] = productId
		}

		//Verifica se a sessão já foi adicionada para evitar repetições
		strUserSession := column[USER_SESSION]
		_, exists = addedEvents[strUserSession]
		if !exists {
			event := BuildEvent(column)
			AddEvent(event)
			addedEvents[strUserSession] = eventId
		}
	}
}

func CalcPercentage(parte, total float64) float64 {
	if total == 0 {
		return 0 // Evita divisão por zero
	}
	return (parte / total) * 100
}

func CalculatePercentageOfOcurrences(part Action, total Action) float64 {

	partMetric, _ := SearchActionMetrics(ACTION_METRICS_FILE, part)
	totalMetric, _ := SearchActionMetrics(ACTION_METRICS_FILE, total)

	fmt.Printf("\n\nParte: %s, Binario: %v, Ocorrencias: %d\n",
		getActionName(part),
		part,
		partMetric.NumberOfOcurrences,
	)
	fmt.Printf("Total: %s, Binario: %v, Ocorrencias: %d\n",
		getActionName(total),
		total,
		totalMetric.NumberOfOcurrences,
	)
	return CalcPercentage(float64(partMetric.NumberOfOcurrences), float64(totalMetric.NumberOfOcurrences))
}
func main() {

	// PopularArquivos()
	ImportarCSV("test.csv")

	fmt.Printf("\n")
	offset, found := BinarySearchOnDisk(PRODUCT_INDEX_FILE, 3)
	if found {
		fmt.Printf("Registro encontrado\n")
		product := ReadFromDataFile[Product](PRODUCT_DATA_FILE, offset)
		fmt.Printf(
			"{ID: %d, CategoryID: %d, Brand: %s, Price: %.2f, Active: %t}\n",
			product.ID,
			product.CategoryID,
			product.Brand,
			product.Price,
			product.Active,
		)
	} else {
		fmt.Printf("Registro não encontrado\n")
	}

	viewMetrics, err := SearchActionMetrics(ACTION_METRICS_FILE, VIEW)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ocorrências para a métrica %s: %d\n",
		getActionName(VIEW),
		viewMetrics.NumberOfOcurrences,
	)

	purchaseMetrics, err := SearchActionMetrics(ACTION_METRICS_FILE, PURCHASE)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ocorrências para a métrica %s: %d\n",
		getActionName(PURCHASE),
		purchaseMetrics.NumberOfOcurrences,
	)

	cartMetrics, err := SearchActionMetrics(ACTION_METRICS_FILE, CART)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ocorrências para a métrica %s: %d\n",
		getActionName(CART),
		cartMetrics.NumberOfOcurrences,
	)

	removeFromCartMetrics, err := SearchActionMetrics(ACTION_METRICS_FILE, REMOVE_FROM_CART)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ocorrências para a métrica %s: %d\n",
		getActionName(REMOVE_FROM_CART),
		removeFromCartMetrics.NumberOfOcurrences,
	)
	fmt.Printf("\n\n")
	mostExpensiveProduct, err := SearchMostExpensiveProduct(MOST_EXPENSIVE_PRODUCT_FILE)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Dados produto mais caro:")
	fmt.Printf(
		"{ID: %d, CategoryID: %d, Brand: %s, Price: %.2f, Active: %t}\n",
		mostExpensiveProduct.ID,
		mostExpensiveProduct.CategoryID,
		mostExpensiveProduct.Brand,
		mostExpensiveProduct.Price,
		mostExpensiveProduct.Active,
	)
	fmt.Printf("\n\n\n")
	fmt.Printf("Listando todos os produtos registrados:\n")
	PrintAllProducts(PRODUCT_DATA_FILE)

	RemoveProduct(PRODUCT_DATA_FILE, PRODUCT_INDEX_FILE, MOST_EXPENSIVE_PRODUCT_FILE, 1)
	fmt.Printf("\nRegistro excluído\n")
	mostExpensiveProduct, _ = SearchMostExpensiveProduct(MOST_EXPENSIVE_PRODUCT_FILE)
	fmt.Printf(
		"Produto mais caro: {ID: %d, CategoryID: %d, Brand: %s, Price: %.2f, Active: %t}\n\n",
		mostExpensiveProduct.ID,
		mostExpensiveProduct.CategoryID,
		mostExpensiveProduct.Brand,
		mostExpensiveProduct.Price,
		mostExpensiveProduct.Active,
	)

	PrintAllProducts(PRODUCT_DATA_FILE)
	PrintAllCategorys(CATEGORY_DATA_FILE)
	PrintAllEvents(EVENT_DATA_FILE)
	// var categoryType Category
	// RemoveByID(CATEGORY_INDEX_FILE, CATEGORY_DATA_FILE, "temp_category.bin", 3, categoryType)
	// PrintAllCategorys(CATEGORY_DATA_FILE)
	fmt.Printf("Abandono de carrinho: %.2f", CalculatePercentageOfOcurrences(REMOVE_FROM_CART, CART))
}
