Of course. Here is a comprehensive `README.md` file for your project, along with a guide on how to create a Postman test suite to validate its functionality.

-----

### \#\# 1. `README.md` File

You can copy and paste the following content directly into a `README.md` file in the root of your `heliosdb` project.

````markdown
# HeliosDB - A Distributed ACID Key-Value Store in Go

HeliosDB is a fault-tolerant, durable, and transactional distributed key-value store built from scratch in Go. It serves as a learning project to demonstrate the core principles of distributed systems, including the Raft consensus algorithm, Write-Ahead Logging for persistence, and ACID transactions using Optimistic Concurrency Control.

## Features

- **Distributed Consensus:** Uses the `hashicorp/raft` library for automatic leader election and fault tolerance.
- **Durable Storage:** Implements a Write-Ahead Log (WAL) to ensure data survives full cluster restarts.
- **Two-Mode API:**
    - A fast, non-transactional API for simple key-value operations.
    - An explicit, fully ACID-compliant transactional API for critical operations.
- **In-Memory Performance:** All data is served from memory for maximum speed.

## Prerequisites

- **Go:** Version 1.20 or later.

## How to Run HeliosDB

Follow these steps to set up and run a 3-node cluster on your local machine.

### Step 1: Clean Up (Optional)

If you have run the project before, start fresh by deleting the old data directories:

```sh
rm -rf node1 node2 node3
````

### Step 2: Create Directories and Config Files

Create the directories for each node and place a configuration file inside each one.

```sh
mkdir -p node1 node2 node3
```

**File: `node1/config.toml`**

```toml
node_id = "node1"
host = "localhost"
port = 8081
raft_port = 9081
data_dir = "."
```

**File: `node2/config.toml`**

```toml
node_id = "node2"
host = "localhost"
port = 8082
raft_port = 9082
data_dir = "."
```

**File: `node3/config.toml`**

```toml
node_id = "node3"
host = "localhost"
port = 8083
raft_port = 9083
data_dir = "."
```

### Step 3: Start the Cluster

Open three separate terminal windows. In each one, `cd` into the respective directory and run the server.

**Terminal 1 (Bootstrap the Leader):**

```sh
cd node1
go run ../cmd/heliosdb/ --bootstrap
```

This node will start and elect itself the leader of a 1-node cluster.

**Terminal 2 (Start Follower 1):**

```sh
cd node2
go run ../cmd/heliosdb/
```

**Terminal 3 (Start Follower 2):**

```sh
cd node3
go run ../cmd/heliosdb/
```

At this point, you have one leader and two followers waiting to be joined.

### Step 4: Form the Cluster

Open a fourth terminal. Use `curl` to send `join` requests to the leader node (`8081`).

**Join node2:**

```sh
curl -X POST -H "Content-Type: application/json" -d '{"node_id": "node2", "addr": "localhost:9082"}' http://localhost:8081/join
```

**Join node3:**

```sh
curl -X POST -H "Content-Type: application/json" -d '{"node_id": "node3", "addr": "localhost:9083"}' http://localhost:8081/join
```

Your 3-node cluster is now fully formed, healthy, and ready to accept requests.

## API Usage

### Simple Key-Value Operations

**Set a value:**

```sh
curl -X POST -d '{"value":"hello world"}' http://localhost:8081/kv/mykey
```

**Get a value (from any node):**

```sh
curl http://localhost:8082/kv/mykey
```

**Delete a value:**

```sh
curl -X DELETE http://localhost:8081/kv/mykey
```

### ACID Transaction Operations

**1. Begin a transaction and get a transaction ID:**

```sh
curl -X POST http://localhost:8081/tx/begin
```

> **Response:** `{"tx_id":"some-unique-id"}`

**2. Stage multiple writes within the transaction (use the `tx_id` from above):**

```sh
curl -X POST -d '{"value":"account A"}' 'http://localhost:8081/tx/set?tx_id=some-unique-id&key=user1'
curl -X POST -d '{"value":"account B"}' 'http://localhost:8081/tx/set?tx_id=some-unique-id&key=user2'
```

**3. Commit the transaction:**

```sh
curl -X POST 'http://localhost:8081/tx/commit?tx_id=some-unique-id'
```

**4. Verify both keys were written atomically:**

```sh
curl http://localhost:8083/kv/user1
curl http://localhost:8083/kv/user2
```

-----

### \#\# 2. Postman Tests Guide

Here’s how to set up a collection in Postman to test your transactional API, including how to automatically handle the `tx_id`.

#### **Step 1: Create a Collection**

1.  In Postman, click "New" \> "Collection".
2.  Name it "HeliosDB API".

#### **Step 2: Create the "Begin Transaction" Request**

1.  Inside your new collection, click "Add a request".
2.  Name the request `1. Begin Transaction`.
3.  Set the method to `POST` and the URL to `http://localhost:8081/tx/begin`.
4.  Go to the **"Tests"** tab for this request. This is where the magic happens. Paste the following JavaScript code:
    ```javascript
    // This code runs *after* the request is sent.
    // It parses the JSON response and saves the tx_id to a collection variable.
    var jsonData = pm.response.json();
    pm.collectionVariables.set("tx_id", jsonData.tx_id);
    console.log("Transaction ID set to: " + jsonData.tx_id);
    ```
5.  Save the request.

#### **Step 3: Create the "Stage Write" Requests**

1.  Create a new request named `2. Stage Write 1`.
2.  Set the method to `POST`.
3.  Set the URL to: `http://localhost:8081/tx/set?tx_id={{tx_id}}&key=tx_postman_key1`
      * The `{{tx_id}}` syntax tells Postman to use the collection variable we saved in the previous step.
4.  Go to the **"Body"** tab, select **"raw"**, and choose **"JSON"**. Enter the following:
    ```json
    {
        "value": "postman atomic write 1"
    }
    ```
5.  Save the request.
6.  Duplicate this request, name it `3. Stage Write 2`, and change the URL and body to use `tx_postman_key2` and a different value.

#### **Step 4: Create the "Commit Transaction" Request**

1.  Create a new request named `4. Commit Transaction`.
2.  Set the method to `POST`.
3.  Set the URL to: `http://localhost:8081/tx/commit?tx_id={{tx_id}}`

#### **Step 5: Run the Test**

Now you can run the requests in order.

1.  Send the `1. Begin Transaction` request. Check the Postman Console (`View > Show Postman Console`) to see the "Transaction ID set to..." message.
2.  Send the `2. Stage Write 1` and `3. Stage Write 2` requests.
3.  Send the `4. Commit Transaction` request.

Finally, use a simple `GET` request (or `curl`) to verify that `tx_postman_key1` and `tx_postman_key2` were written correctly. This setup allows you to easily test your entire transactional flow with a single click sequence.