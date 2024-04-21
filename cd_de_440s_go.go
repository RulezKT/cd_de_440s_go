package cd_de_440s_go

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/RulezKT/cd_cheb_go"
	"github.com/RulezKT/cd_consts_go"
	"github.com/RulezKT/cd_date_go"
	"github.com/RulezKT/cd_nodes_go"
)

func Load440s() cd_consts_go.BspFile {

	var bsp cd_consts_go.BspFile

	fileName := cd_consts_go.FILENAME
	expectedSha512 := cd_consts_go.EXPECTEDSHA512
	fileLength := cd_consts_go.FILELENGTH
	bsp.FileInfo = cd_consts_go.De440sFile()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	pathToFile := filepath.Join(dir, "files", fileName)
	bsp.FilePtr = CheckAndOpen(pathToFile, expectedSha512, fileLength)
	bsp.NodesCoords = cd_nodes_go.LoadNodesCoords()

	bsp.DeltaTTable = cd_date_go.DeltaTPtr()

	return bsp
}

// максимальная и минимальные даты в файле, измеряется в секундах от J2000
// J2000 - 2000 Jan 1.5 (12h on January 1) or JD 2451545.0 - TT time or
// January 1, 2000, 11:58:55.816 UTC (Coordinated Universal Time).
// = 0 in seconds

//	SEGMENT_START_TIME := 0.0 // start time of segment in our case only 1 segment os of the whole file
//	 SEGMENT_LAST_TIME := 0 // end time of segment in our case only 1 segment os of the whole file

//	var  summaryRecordStruct SummaryRecordStruct
//	var  summariesLineStruct SummariesLinesTemplate
/*
   //Для каждого файла своя инициализация этой структуры, адреса начала данных для каждой планеты
   initializeSummariesStruct();
*/

// Расчёт координат положений планет на указанную дату (секунды с J2000)
// есть проверка на Range of dates и на нахождение сегмента данных
func GetCoordinates(dateInSeconds int64, targetCode int, centerCode int, bspFile cd_consts_go.BspFile) cd_consts_go.Position {
	// gets barycentric equatorial Cartesian positions and velocities
	// relative to the equinox 2000/ICRS

	//В системе Geocentric Equatorial
	// ось X фиксирована в направлении на Солнце в момент весеннего равноденствия
	// (the first point of Aries (i.e. the position of the sun at the vernal equinox).
	// Это направление есть пересечение плоскостей земного экватора и Эклиптики.
	// Ось Z параллельна оси вращения Земли и Y дополняет ортагональную систему правой руки (Y = Z x X).

	//в файлах все данные - экваториальные картезианские
	//соответственно единственный вариант:
	//считаем Планеты относительно Земли
	//переводим в эклиптические координаты, используя расчитанный EPS

	//в одном файле время  начала и конца везде одинаковое

	dataReader := bspFile.FilePtr
	fi := bspFile.FileInfo

	SEGMENT_START_TIME := fi.SummariesLineStruct[1].SEGMENT_START_TIME
	SEGMENT_LAST_TIME := fi.SummariesLineStruct[1].SEGMENT_LAST_TIME

	if dateInSeconds <= SEGMENT_START_TIME || dateInSeconds >= SEGMENT_LAST_TIME {

		log.Fatal("getCoordinates: Date is out of range")
		return cd_consts_go.Position{}
	}

	arrayInfoOffset := 0
	internalOffset := 0

	var arrayInfo cd_consts_go.ArrayInfo

	var pos cd_consts_go.Position

	//0 пропускаем -это SOLAR SYSTEM BARYCENTER, там все данные 0
	for i := 1; i <= fi.SummaryRecordStruct.TotalSummariesNumber; i++ {
		if fi.SummariesLineStruct[i].TargetCode == targetCode && fi.SummariesLineStruct[i].CenterCode == centerCode && SEGMENT_START_TIME < dateInSeconds && SEGMENT_LAST_TIME > dateInSeconds {

			/*
			   The records within a segment are ordered by increasing initial epoch. All records contain the same number
			   of coefficients. A segment of this type is structured as follows:

			   +---------------+
			   | Record 1      |
			   +---------------+
			   | Record 2      |
			   +---------------+
			   .
			   .
			   .
			   +---------------+
			   | Record N      |
			   +---------------+
			   | INIT          |
			   +---------------+
			   | INTLEN        |
			   +---------------+
			   | RSIZE         |
			   +---------------+
			   | N             |
			   +---------------+
			*/

			// встаем на позицию 4 слова до конца summaries_line_struct[i] там есть 4 нужных нам double
			arrayInfoOffset = (fi.SummariesLineStruct[i].RecordLastAddress - 4) * 8
			/* читаем
			   double init - start time of the first record in array
			   double intlen - the length of one record (seconds)
			   double rsize - number of elements in one record
			   double n - number of records in segment
			*/

			/*
			   данные записаны по типу LITTLE_ENDIAN, настраиваем буфер соответственно
			   https://docs.oracle.com/en/java/javase/12/docs/api/java.base/java/nio/ByteBuffer.html
			   https://www.geeksforgeeks.org/bytebuffer-order-method-in-java-with-examples/
			   java.nio.ByteBuffer
			   public final ByteOrder order()
			*/
			//ByteOrder originalOrder = dataReader.order();
			//dataReader.order(ByteOrder.LITTLE_ENDIAN);

			dataReader.Seek(int64(arrayInfoOffset), io.SeekStart)
			// arrayInfo.init = dataReader.getDouble();
			byteArr := make([]byte, 8)
			dataReader.Read(byteArr)
			arrayInfo.Init = math.Float64frombits(binary.LittleEndian.Uint64(byteArr[0:]))
			//fmt.Println(arrayInfo.Init)

			//arrayInfo.intlen = dataReader.getDouble(arrayInfoOffset + 8);
			byteArr = make([]byte, 8)
			dataReader.Read(byteArr)
			arrayInfo.Intlen = math.Float64frombits(binary.LittleEndian.Uint64(byteArr[0:]))
			//fmt.Println(arrayInfo.Intlen)

			//arrayInfo.rsize = dataReader.getDouble(arrayInfoOffset + 16);
			byteArr = make([]byte, 8)
			dataReader.Read(byteArr)
			arrayInfo.Rsize = math.Float64frombits(binary.LittleEndian.Uint64(byteArr[0:]))
			//fmt.Println(arrayInfo.Rsize)

			//arrayInfo.n = dataReader.getDouble(arrayInfoOffset + 24);
			byteArr = make([]byte, 8)
			dataReader.Read(byteArr)
			arrayInfo.N = math.Float64frombits(binary.LittleEndian.Uint64(byteArr[0:]))
			//fmt.Println(arrayInfo.N)

			// находим смещение на нужную запись внутри массива
			// округляется вниз до конца предыдущей записи
			// internalOffset = Math.floor((dateInSeconds - arrayInfo.init) / arrayInfo.intlen)*Math.trunc(arrayInfo.rsize);
			//internalOffset = (int) (Math.floor((dateInSeconds - arrayInfo.init) / arrayInfo.intlen) * ((int) arrayInfo.rsize));
			internalOffset = int(math.Floor((float64(dateInSeconds)-arrayInfo.Init)/arrayInfo.Intlen)) * int(arrayInfo.Rsize)

			// встаем на начало нужной записи
			record := int(8 * (fi.SummariesLineStruct[i].RecordStartAddress + internalOffset))

			/* The first two elements in the record, MID and RADIUS, are the midpoint and radius
			   of the time interval covered by coefficients in the record.
			       These are used as parameters to perform transformations between
			   the domain of the record (from MID - RADIUS to MID + RADIUS)
			   and the domain of Chebyshev polynomials (from -1 to 1 ).
			   The same number of coefficients is always used for each component,
			       and all records are the same size (RSIZE),
			       so the degree of each polynomial is
			   ( RSIZE - 2 ) / 3 - 1
			   // the degree of the polynomial is number_of_coefficients-1
			   но по факту почему то используется ( RSIZE - 2 ) / 3 ????
			*/

			// final double[] array_of_coffs = new double[(int) arrayInfo.Rsize];
			array_of_coffs := make([]float64, (int)(arrayInfo.Rsize))

			// почему то стартовый адрес первой записи на 1 слово больше чем надо, поэтому вычитаем 8
			start_record := int(record - 8)
			// fread(array_of_coffs, (int) (array_info.rsize * 8), 1, bsp_430_file);
			//читаем arrayInfo.rsize коэффициентов
			dataReader.Seek(int64(start_record), io.SeekStart)
			for k := 0; k < int(arrayInfo.Rsize); k++ {
				//array_of_coffs[k] = dataReader.getDouble()
				byteArr = make([]byte, 8)
				dataReader.Read(byteArr)
				array_of_coffs[k] = math.Float64frombits(binary.LittleEndian.Uint64(byteArr[0:]))
				//fmt.Println(array_of_coffs[k])
			}

			/*
			   Начинаем расчёты полиномов Чебышева
			*/
			order := int((arrayInfo.Rsize-2)/3 - 1)
			tau := (float64(dateInSeconds) - array_of_coffs[0]) / array_of_coffs[1]
			// float beg = 2;
			// float end = beg + order + 1;
			// order = (int) (order);
			deg := int(order + 1)
			factor := 1.0 / array_of_coffs[1] // unscale time dimension

			// от 0 до arrayInfo.rsize набор double
			//final double[] array_of_coffs = new double[(int) arrayInfo.rsize];
			//let coffs_ptr = array_of_coffs.slice(2, 2 + deg);
			//var coffs_ptr []double = Arrays.copyOfRange(array_of_coffs, 2, 2 + deg);
			coffs_ptr := array_of_coffs[2 : 2+deg]
			x := cd_cheb_go.Chebyshev(order, tau, coffs_ptr) // array_of_coffs[2:2 + deg]);
			pos.X = x

			//coffs_ptr = array_of_coffs.slice(2 + deg, 2 + 2 * deg);
			//coffs_ptr = Arrays.copyOfRange(array_of_coffs, 2 + deg, 2 + 2 * deg);
			coffs_ptr = array_of_coffs[2+deg : 2+2*deg]
			y := cd_cheb_go.Chebyshev(order, tau, coffs_ptr) // array_of_coffs[2 + deg:2 + 2 * deg]);
			pos.Y = y

			//coffs_ptr = array_of_coffs.slice(2 + 2 * deg, 2 + 3 * deg);
			//coffs_ptr = Arrays.copyOfRange(array_of_coffs, 2 + 2 * deg, 2 + 3 * deg);
			coffs_ptr = array_of_coffs[2+2*deg : 2+3*deg]
			z := cd_cheb_go.Chebyshev(order, tau, coffs_ptr) // array_of_coffs[2 + 2 * deg:2 + 3 * deg]);
			pos.Z = z

			// type 2 uses derivative on the same polynomial
			//coffs_ptr = array_of_coffs.slice(2, 2 + deg);
			//coffs_ptr = Arrays.copyOfRange(array_of_coffs, 2, 2 + deg);
			coffs_ptr = array_of_coffs[2 : 2+deg]
			velocityX := cd_cheb_go.DerChebyshev(order, tau, coffs_ptr) * factor // array_of_coffs[2:2 + deg]) * factor;
			pos.VelocityX = velocityX

			//coffs_ptr = array_of_coffs.slice(2 + deg, 2 + 2 * deg);
			// coffs_ptr = Arrays.copyOfRange(array_of_coffs, 2 + deg, 2 + 2 * deg);
			coffs_ptr = array_of_coffs[2+deg : 2+2*deg]
			velocityY := cd_cheb_go.DerChebyshev(order, tau, coffs_ptr) * factor // array_of_coffs[2 + deg:2 + 2 * deg]) * factor;
			pos.VelocityY = velocityY

			//coffs_ptr = array_of_coffs.slice(2 + 2 * deg, 2 + 3 * deg);
			//coffs_ptr = Arrays.copyOfRange(array_of_coffs, 2 + 2 * deg, 2 + 3 * deg);
			coffs_ptr = array_of_coffs[2+2*deg : 2+3*deg]
			velocityZ := cd_cheb_go.DerChebyshev(order, tau, coffs_ptr) * factor // array_of_coffs[2 + 2 * deg:2 + 3 * deg]) * factor;
			pos.VelocityZ = velocityZ

			return pos
		}

	}

	log.Fatal("getCoordinates: Date is out of range")
	return cd_consts_go.Position{}

}

//открытие и проверка байтовых BSP файлов

func CheckAndOpen(pathToFile string, checkSha512 string,
	fileLength int) *bytes.Reader {

	fileContent, err := os.ReadFile(pathToFile)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	length := len(fileContent)

	if length != fileLength {
		fmt.Println("Something wrong with the file length, file: ", pathToFile)
		fmt.Println("Expected length: ", fileLength)
		fmt.Println("Current length: ", length)
		return nil
	}

	//получить sha512 в windows  certutil -hashfile "de430.bsp" SHA512
	//recieving sha512 of data and comparing with given sha512'
	sha512ByteArr := sha512.Sum512(fileContent) //[Size]byte
	sha512String := hex.EncodeToString(sha512ByteArr[:])
	if sha512String != checkSha512 {
		fmt.Println("Something wrong with checkSha512")
		fmt.Println("sha512String: ", sha512String)
		fmt.Println("checkSha512: ", checkSha512)
		fmt.Println(sha512String == checkSha512)
		return nil
	}

	return bytes.NewReader(fileContent)
}
