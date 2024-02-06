# SmartCrawl

An adaptable framework for gathering, aggregating and analyzing data, focusing on blockchain and smart contracts.

<!-- MarkdownTOC -->

- [SmartCrawl](#smartcrawl)
  - [Project](#project)
  - [Config](#config)
  - [Provider](#provider)
  - [Item](#item)
  - [Container](#container)
  - [Factory](#factory)
  - [Tasks](#tasks)
  - [Queues](#queues)
    - [CLI Commands](#cli-commands)
      - [Launching Workers](#launching-workers)
      - [Executing Individual Tasks](#executing-individual-tasks)
      - [Periodic Actions (Cron)](#periodic-actions-cron)
      - [Cases](#cases)
      - [Tools](#tools)

<!-- /MarkdownTOC -->

## Project

A set of entities (from various providers) grouped by theme. For instance, gathering smart contracts. A single project can include smart contracts from different blockchains. Storing multiple projects in one database table/collection is not supported.

## Config

Project settings are stored in `config.json`. Multi-project support is not provided. Previously considered, but removed for simplicity. If multiple projects are needed, they can run on the same engine in different directories and with different configs. There is no environment separation (test, dev, etc.) in this file. Differences between environments and sensitive information are loaded via environment variables and/or .env files.

## Provider

ItemProvider is an abstraction describing a collection of some items, usually grouped by containers. For example:
1) ItemProvider: software catalog, container: page of some category, item: application/library
2) ItemProvider: blockchain, container: block, item: deployed contracts within block

## Item

An entity, for example, a smart contract or a program/application/library. Items have fields `ProvName`, `ProvBranch` characterizing the provider and its "sub-provider", like a blockchain and its subnets, as well as an `Id` field uniquely defining the entity within the set defined by `ProvName`, `ProvBranch`. For a smart contract, this would be its address.

## Container

A collection containing the sought-after items (items). For blockchain, this could be a block (with transactions/deployed contracts as items), for a website, a page with repeating elements. It's assumed that a container has an ID, which could be composite: block ID, category URL + page number.
* `app/container.go`: ItemsContainer

## Factory

The "factory" package, like main, "knows" about all dependencies, meaning it can import all other packages in the application except main. Dependency graphs:
* main -> app, factory
* factory -> app, asynq, mongo, zilliqa
* asynq -> app
* mongo -> app
* zilliqa -> app

## Tasks

Isolated parts of code located in `app/job/`. For an example, see the container processing task in `app/job/container.go`. Essential parameters include only the task type(name), defined directly in the task files, e.g., `app/job/container.go`. Task names start with `job:`, like `job:container:process`, `job:property:set`. Task code should use only general interfaces and types. Specific action implementations are outsourced to dependencies. A task might use a single provider or none at all. Future might introduce tasks with multiple providers, but this is not currently the case. Tasks are created using constructors (NewJobMessage...), marshaled, and added to the queue. Unmarshaling is handled in `factory/job.php`.

## Queues

Currently, the task queue is based on the package `github.com/hibiken/asynq`, using a Redis backend. This package facilitates adding tasks to the queue and their "consumption" from there, meaning task worker-handlers are implemented using this package. The project adheres to a convention: one task type per queue. Therefore, workers for each type of task (i.e., queue) are run in a separate CLI.
Some operations create an additional queue, for example, when adding a property, see Cases(1).

### CLI Commands

#### Launching Workers

- `go run cmd/main.go --provider=zilmain worker --queue=job:container:process --limit=5` -- **block processing workers** (Zilliqa mainnet)
- `go run cmd/main.go --provider=zilmain worker --queue=job:property:set:{PropertyName} --limit=2` -- **delayed property processing workers** (Zilliqa mainnet)

#### Executing Individual Tasks

- Direct execution of tasks like `job:property:set`, with item ID and property name as parameters.
- Direct execution of `job:container:process`, with container ID as parameter. The algorithm involves extracting entities from the container, filling auto properties, saving entities to the database, returning, and executing tasks for setting delayed properties.

#### Periodic Actions (Cron)

- Searching for new containers starting from the last processed one (stored in state); adding tasks to their queue for processing. Set on a cron, with interval and limit adjusted for each provider.

#### Cases

1. **Adding a new realtime/delayed field to an item** (separately for each provider)
   - First, add the field to the item and the function to fill this field, then call RegisterRealtimeAutosetter/RegisterDelayedAutosetter depending on the field type.
   - Tasks for adding this property to items without it are queued. Continue until additions are made. After adding each task, the corresponding record in the database is marked as processed, for a text field, a blank string value is set. Tasks corresponding to the property name are added to the queue, processed by workers (one or several) launched specifically for each queue.

2. **A field is calculated incorrectly**, a bug is found and fixed, but fields in the database need to be updated.
   - Write a command in mongosh to delete incorrect fields, then follow the algorithm described in (4).

3. **Removing an unnecessary field**: `db.item.updateMany({"provname": "Zilliqa", "provbranch": "1"}, {$unset:{fieldname:""}})`

#### Tools

- `./q.sh dash` launches CLI dashboard for asynq.
- `./q.sh queue list` lists queues.
- `./q.sh queue remove queue-name` removes an empty queue.
- `./dump.sh` -- creates a dump of the Mongo database specified in .env.local->STORAGE_DBNAME in ./backups/.
- `./restore.sh ./_backup/mongodump_07-01-2023_13-33-36.gz` -- restores a dump (including indexes) from a backup, current collections are dropped.
