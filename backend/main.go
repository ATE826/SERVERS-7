package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

// Client представляет клиента WebSocket.
type Client struct {
	conn *websocket.Conn
	// Дополнительные поля, если нужны (например, имя пользователя)
}

// Message представляет сообщение чата.
type Message struct {
	Text string `json:"text"`
	// Дополнительные поля, если нужны (например, отправитель, время)
}

var (
	// clients хранит список всех подключенных WebSocket клиентов.
	clients = make(map[*Client]bool)
	// broadcast используется для отправки сообщений всем клиентам.
	broadcast = make(chan Message)
	// mutex для безопасного доступа к карте clients.
	mutex = &sync.Mutex{}
)

func main() {
	// Запуск обработчика сообщений в отдельной горутине
	go handleMessages()

	// Настройка обработчика WebSocket
	http.Handle("/ws", websocket.Handler(handleWebSocket))

	// Запуск HTTP сервера (для WebSockets)
	go func() {
		fmt.Println("WebSocket сервер запущен на :8080")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal("ListenAndServe (WebSocket): ", err)
		}
	}()

	// Запуск TCP сервера
	go func() {
		fmt.Println("TCP сервер запущен на :8081")
		listener, err := net.Listen("tcp", ":8081")
		if err != nil {
			log.Fatal("Listen (TCP): ", err)
		}
		defer listener.Close()

		for {
			// Принимаем входящие соединения
			conn, err := listener.Accept()
			if err != nil {
				log.Println("Error accepting TCP connection:", err)
				continue
			}
			// Обрабатываем соединение в отдельной горутине
			go handleTCPConnection(conn)
		}
	}()

	// Заставим основную горутину ждать (чтобы серверы не завершились сразу)
	select {}
}

// handleWebSocket обрабатывает новое WebSocket соединение.
func handleWebSocket(ws *websocket.Conn) {
	// Создаем нового клиента
	client := &Client{conn: ws}

	// Добавляем клиента в список
	mutex.Lock()
	clients[client] = true
	mutex.Unlock()

	fmt.Println("Новый WebSocket клиент подключен")

	// Чтение сообщений от клиента
	for {
		var msg Message
		// Читаем сообщение от клиента
		err := websocket.JSON.Receive(ws, &msg)
		if err != nil {
			// Если произошла ошибка (например, клиент отключился), удаляем клиента
			if err != io.EOF {
				log.Printf("Ошибка чтения WebSocket сообщения от клиента %v: %v\n", client.conn.RemoteAddr(), err)
			} else {
				fmt.Printf("WebSocket клиент %v отключен\n", client.conn.RemoteAddr())
			}

			mutex.Lock()
			delete(clients, client)
			mutex.Unlock()
			break // Выходим из цикла чтения
		}

		// Отправляем полученное сообщение в канал broadcast
		broadcast <- msg
	}
}

// handleMessages принимает сообщения из канала broadcast и отправляет их всем клиентам.
func handleMessages() {
	for {
		// Ожидаем новое сообщение из канала broadcast
		msg := <-broadcast

		fmt.Printf("Получено сообщение для рассылки: %s\n", msg.Text)

		// Отправляем сообщение всем подключенным клиентам
		mutex.Lock()
		for client := range clients {
			err := websocket.JSON.Send(client.conn, msg)
			if err != nil {
				log.Printf("Ошибка отправки WebSocket сообщения клиенту %v: %v\n", client.conn.RemoteAddr(), err)
				// Если не удалось отправить, возможно, клиент отключился, удаляем его
				client.conn.Close() // Закрываем соединение
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}

// handleTCPConnection обрабатывает новое TCP соединение.
func handleTCPConnection(conn net.Conn) {
	fmt.Printf("Новое TCP соединение от %s\n", conn.RemoteAddr())
	defer conn.Close() // Убедимся, что соединение закрыто при выходе из функции

	// Чтение данных из соединения
	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Ошибка чтения TCP данных от %s: %v\n", conn.RemoteAddr(), err)
			} else {
				fmt.Printf("TCP клиент %s отключен\n", conn.RemoteAddr())
			}
			break // Выходим из цикла чтения
		}

		// Выводим полученные данные (для примера)
		receivedData := string(buffer[:n])
		fmt.Printf("Получено TCP данных от %s: %s\n", conn.RemoteAddr(), receivedData)

		// Опционально: отправить ответ клиенту
		// response := fmt.Sprintf("Вы прислали: %s", receivedData)
		// conn.Write([]byte(response))
	}
}
