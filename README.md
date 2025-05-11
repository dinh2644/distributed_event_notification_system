# Distributed Event Notification System for E-commerce

This project implements a distributed event notification system for an e-commerce platform. It utilizes a microservice architecture orchestrated by the event notification pattern, specifically employing a choreography saga pattern. The system comprises five distinct services that collaborate to handle order transactions seamlessly.

## Architecture

The system is built around 5 microservices:

1.  **Order Service**: Initiates the order process when a user places an order. It creates an `OrderPlaced` event.
2.  **Payment Service**: Listens for `OrderPlaced` events. Upon successful processing, it broadcasts a `PaymentConfirmed` event.
3.  **Inventory Service**: Listens for `PaymentConfirmed` events. It updates the inventory status and then broadcasts an `InventoryUpdated` event.
4.  **Shipping Service**: Listens for `InventoryUpdated` events. It processes the shipping details and broadcasts a `ShippingConfirmed` event.
5.  **Notification Service**: Listens for `ShippingConfirmed` events. It is responsible for sending a shipping confirmation email to the user via SMTP.

Events are formatted according to the **CloudEvent** standard and are broadcast to a **single Kafka topic and partition**. All services subscribe to this topic and listen for relevant events, triggering subsequent actions in the saga.

![image](https://github.com/user-attachments/assets/f27e460b-a44a-4354-952c-a4b67f1e817f)


## Technologies Used

* **Go (Golang)**: For building the microservices.
* **Apache Kafka**: As the distributed event streaming platform.
* **Redis**: Used for application-level message ordering and duplication prevention.
    * **Message Ordering**: The order of messages is managed by associating a state with each order transaction, indicating its progress through the saga.
    * **Deduplication**: Processed `orderId`s are stored as keys in Redis, providing a quick in-memory cache to prevent processing duplicate messages.
* **Postman**: For simulating user requests (placing an order).
* **Docker & Docker Compose**: For containerizing and orchestrating the application environment.
* **CloudEvents**: Standard for describing event data in a common way.
* **SMTP (Gmail)**: For sending email notifications.

## Current State

This project currently simulates the "happy path" of an order transaction, where all events proceed successfully, culminating in the Notification Service sending a shipping confirmation email.

**Limitations:**
* Error handling for failed events (e.g., `OrderCancelled`, `PaymentNotAuthorized`, insufficient inventory, shipping issues) is not yet implemented. The primary focus of this version is to demonstrate the event notification flow in a distributed system.

## Getting Started

### Prerequisites

* Docker
* Go
* Gmail account (if you want to test the email notification feature, go to your Google's PFP icon, Security, under `How you sign in to Google`, click `2-Step Verification`, scroll down and create app password under `App passwords`, this will be your `GMAIL_APP_PASSWORD` in `.env` file) 

### Installation & Running

1.  **Clone the repository:**
    ```bash
    # git clone <your-repository-url>
    # cd <your-project-directory>
    ```

2.  **Build the services:**
    This command will compile the Go binaries for each service.
    ```bash
    make build
    ```

3.  **Run `go mod tidy` (Optional but Recommended):**
    To ensure all Go module dependencies are correctly managed, run:
    ```bash
    make tidy
    ```

4.  **Start the services:**
    This command will start all services (Kafka, Redis, and the five application microservices) using Docker Compose. It will also build the Docker images if they don't exist or if changes are detected.
    ```bash
    make start
    ```
    **Note:** Startup may take a few seconds as the system checks for the healthy status of prerequisite services like Kafka and Redis before the application services fully initialize. If there are any error, run `make start` again

6.  **Check Logs:**
    You can monitor the startup process and runtime logs of all services via Docker Desktop or by using the following command in your terminal:
    ```bash
    make logs
    ```
    Look for messages indicating that each service has connected to Kafka and is ready to process events.

### Other Useful Commands

* **Stop the services:**
    ```bash
    make stop
    ```
    
* **Restart the services:**
    This command will stop all services, rebuild the Go binaries and Docker images, and then start the services again.
    ```bash
    make restart
    ```

* **Clean build artifacts:**
    ```bash
    make clean
    ```
    This will remove the compiled Go binaries from the `build/` directory.

## Usage

Once all services are running (check `make logs` or Docker Desktop), you can simulate placing an order using Postman:

1.  Open Postman.
2.  Create a new **POST** request.
3.  Set the request URL to the Order Service endpoint (e.g., `http://localhost:8080/create-order` - *Please verify the correct port for your Order Service if it's different*).
4.  Go to the **Body** tab, select **raw**, and choose **JSON** from the dropdown.
5.  Paste the following JSON payload:

    ```json
    {
      "userId": "a7f3e2c9-8b1f-4d23-bb0f-9a1e97c1a3b2",
      "userEmail": "your-email@example.com",
      "item": "keyboard",
      "amount": 49.99
    }
    ```
    *(Replace `"your-email@example.com"` with an email address you can access to receive the shipping confirmation.)*

6.  Click **Send**.
7.  Check `localhost:8085` if you want to see the Kafka GUI

This will trigger the `OrderPlaced` event, initiating the saga. You can observe the event flow through the logs (`make logs`) and should eventually receive a shipping confirmation email if the Notification Service is correctly configured.

---