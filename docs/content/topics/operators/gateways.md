---
title: Gateways
description: How gateways work and how to create your own.
section: operators
weight: 4
aliases:
  - /gateways.md
  - /topics/gateways.md
  - /topics/operators/gateways.md
---

This guide explains how gateways work, and provides guidance for creating your
own.

## What Is A Brigade Gateway?

The [Brigade architecture](/topics/design) is oriented around the concept that
Brigade scripts run as a response to one or more events. In Brigade, a gateway
is an entity that generates events from external sources.

Currently, all of our officially supported gateways are triggered by webhooks
delivered from external systems over HTTP(S), but there are no rules about what
can be used as a trigger for an event. A gateway could hypothetically listen on
a message queue, run as a chat bot, or watch files on a file system.

While Brigade ships without any gateways included, installing one alongside
Brigade is as simple as a [Helm] chart install, along with any additional setup
particular to a given gateway. See [below](#available-gateways) for a current
listing of compatible gateways.

[Helm]: https://helm.sh

## Available Gateways

Currently, the list of official Brigade gateways is as follows:

* [ACR (Azure Container Registry) Gateway](https://github.com/brigadecore/brigade-acr-gateway)
* [BitBucket Gateway](https://github.com/brigadecore/brigade-bitbucket-gateway/tree/v2)
* [CloudEvents Gateway](https://github.com/brigadecore/brigade-cloudevents-gateway)
* [Docker Hub Gateway](https://github.com/brigadecore/brigade-dockerhub-gateway)
* [GitHub Gateway](https://github.com/brigadecore/brigade-github-gateway)
* [Slack Gateway](https://github.com/brigadecore/brigade-slack-gateway)

Follow the installation instructions provided in each gateway's repository to
learn how to get started.

### The Anatomy of a Brigade Event

All gateways perform the same job, that is, to translate activity and context
from some source into a Brigade event. Let's now look at the structure of a
Brigade event.

A Brigade Event is defined primarily by its source and type values, worker
configuration and worker status.

Here is a YAML representation of an event created via the `brig event create`
command for the [01-hello-world sample project]. Note that many of the fields
shown are populated with system-generated values, such as timestamps, IDs and
worker status. The primary fields that can be set by a client (a gateway in
this case) are `projectID`, `source` and `type`.

```yaml
apiVersion: brigade.sh/v2
kind: Event
metadata:
  created: "2021-08-11T22:22:41.366Z"
  id: 48c960eb-5823-46d0-8390-ec6a2a966b98
projectID: hello-world
source: brigade.sh/cli
type: exec
worker:
  spec:
    configFilesDirectory: .brigade
    defaultConfigFiles:
      brigade.js: |
        console.log("Hello, World!");
    logLevel: DEBUG
    useWorkspace: false
    workspaceSize: 10Gi
  status:
    apiVersion: brigade.sh/v2
    ended: "2021-08-11T22:22:49Z"
    kind: WorkerStatus
    phase: SUCCEEDED
    started: "2021-08-11T22:22:41Z"
```

Let's look at the high-level sections in the event definition above.  They are:

  1. Event metadata, including:
    i. The `apiVersion` of the schema for this event
    ii. The schema `kind`, which will always be `Event`
    iii. The `id` of the Event
    iv. A `created` timestamp for the Event
    v. The `projectID` that the Event is associated with
    vi. The event `source`
    vii. The event `type`
  2. The `worker.spec` section, which contains worker configuration inherited
    from the project definition associated with the event in combination with
    system-level defaults.
  3. The `worker.status` section, which contains the `started` and `ended`
    timestamps and current `phase` of the worker handling the event. In the
    example above, it has already reached the terminal phase of `SUCCEEDED`.

To explore the SDK definitions of an Event object, see the [Go SDK Event] and
[JavaScript/TypeScript SDK Event].

[01-hello-world sample project]: https://github.com/brigadecore/brigade/tree/main/examples/01-hello-world
[Go SDK Event]: https://github.com/brigadecore/brigade/blob/main/sdk/v3/events.go
[JavaScript/TypeScript SDK Event]: https://github.com/brigadecore/brigade-sdk-for-js/blob/main/src/core/events.ts

## Creating Custom Gateways

Given the above description of how gateways work, we can walk through how a
minimal example gateway can be built. We'll focus on the event creation side
of a Brigade gateway, rather than going over other common attributes, such as
an HTTP(S) server that awaits external webhook events.

Since the Brigade API server is the point of contact for gateway
authentication/authorization and event submission, gateway developers will need
to pick an [SDK] to use. For this example, we'll be using the [Go SDK]. As of
this writing, a [Javascript/Typescript SDK] and a [Rust SDK] (work in
progress) also exist.

[SDK]: https://github.com/brigadecore/brigade/tree/main#sdks
[Go SDK]: https://github.com/brigadecore/brigade/tree/main/sdk
[Javascript/Typescript SDK]: https://github.com/brigadecore/brigade-sdk-for-js
[Rust SDK]: https://github.com/brigadecore/brigade-sdk-for-rust


## Events and Sensitive Information

Before proceeding further, we're obliged to mention that [Events] emitted by a
gateway should **NEVER** contain secret or sensitive information. Because
Brigade routes events to interested parties ([projects]) based on a
subscription model, always assume that any project in your Brigade instance
could be subscribed to any event that a gateway creates.

In practice, this shouldn't be a difficult thing to overcome. Events can
contain non-secret references to things that parties (projects) having
appropriate secrets can access. By way of example, anyone can subscribe to
events from the [GitHub gateway] originating from any repo -- even private ones
-- but only projects having the correct secrets will ever be able to pull
source from such a repo.

Otherwise, operators also have the choice of installing a separate, private
Brigade instance with its own gateway array. See the [Deployment] doc for
guidance on how to deploy more than one Brigade instance to a Kubernetes
cluster.

[Events]: /topics/project-developers/events
[projects]: /topics/project-developers/projects
[GitHub gateway]: https://github.com/brigadecore/brigade-github-gateway
[Deployment]: /topics/operators/deployment#deploying-multiple-brigade-instances

## Example Gateway

The following example assumes a running Brigade instance has been deployed and
the ability to create a service account is in place (e.g. you have the role of
'ADMIN' or you are logged in as the root user). If you'd like to follow along
and haven't yet deployed Brigade, check out the [QuickStart].

[QuickStart]: /intro/quickstart

### Preparation

#### Service Account creation

All Brigade gateways require a service account token for authenticating with
Brigade when submitting an event into the system. As preparation, we'll create
a service account for this gateway and save the generated token for use in our
program.

```shell
$ brig service-account create \
	--id example-gateway \
	--description example-gateway
```

Make note of the token returned. This value will be used in another step. It is
your only opportunity to access this value, as Brigade does not save it.

Authorize this service account to create new events of a given source:

```shell
$ brig role grant EVENT_CREATOR \
    --service-account example-gateway \
    --source example.org/example-gateway
```

Note: The `--source example.org/example-gateway` option specifies that this
service account can be used only to create events having a value of
`example.org/example-gateway` in the event's `source` field. This is a security
measure that prevents the gateway from using this token for impersonating other
gateways.

The rule of thumb to avoid `source` clashes is to use a URI you control. This
means leading with one's own domain or the URL for something else one owns,
like the URL for a GitHub repo, for instance.

#### Go setup

We'll be using the [Go SDK] for our example gateway program and we'll need to
do a bit of prep. We're assuming your system has Go installed and configured
properly. (If not, please visit the [Go installation docs] to do so.)

Let's create a directory where our program's `main.go` file can reside and
perform bootstrapping for our Go program, including initializing its Go module
and fetching the Brigade SDK dependency:

```shell
$ mkdir example-gateway
$ cd example-gateway
$ go mod init example-gateway
$ go get github.com/brigadecore/brigade/sdk/v3
$ touch main.go
```

[Go installation docs]: https://golang.org/doc/install

### Example Gateway code

Now we're ready to code! Open `main.go` in the editor of your choice and add in
the following Go code.

The program consists of a `main` function which procures the Brigade API server
address and the gateway token (generated above) via environment variables. It
then constructs an API client from these values and passes this to the
`createEvent` helper function. This function builds a Brigade Event with the
pertinent fields populated and then calls the SDK's event create function.

See the in-line comments for further description around each section.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/brigadecore/brigade/sdk/v3"
	"github.com/brigadecore/brigade/sdk/v3/restmachinery"
)

func main() {
	ctx := context.Background()

	// Get the Brigade API server address and token from the environment
	apiServerAddress := os.Getenv("APISERVER_ADDRESS")
	if apiServerAddress == "" {
		log.Fatal("Required environment variable APISERVER_ADDRESS not found.")
	}
	gatewayToken := os.Getenv("API_TOKEN")
	if gatewayToken == "" {
		log.Fatal("Required environment variable API_TOKEN not found.")
	}

	// The default Brigade deployment mode uses self-signed certs
	// Hence, we allow insecure connections in our APIClientOptions
	// This can be changed to false if insecure connections should not be allowed
	apiClientOpts := &restmachinery.APIClientOptions{
		AllowInsecureConnections: true,
	}

	// Create an API client with the gateway token value
	client := sdk.NewAPIClient(
		apiServerAddress,
		gatewayToken,
		apiClientOpts,
	)

	// Construct a Brigade Event
	event := sdk.Event{
		// This is the source value for this event
		Source: "example.org/example-gateway",
		// This is the event's type
		Type: "hello",
		// This is the event's payload
		Payload: "Dolly",
	}

	// Create the Brigade Event
	events, err := client.Core().Events().Create(ctx, event, nil)
	if err != nil {
		log.Fatal(err)
	}

	// If the returned events list has no items, no event was created
	if len(events.Items) != 1 {
		fmt.Println("No event was created.")
		return
	}

	// The Brigade event was successfully created!
	fmt.Printf("Event created with ID %s\n", events.Items[0].ID)
}
```

Let's briefly look at the Brigade Event object from above.

```go
  // Construct a Brigade Event
  event := sdk.Event{
    // This is the source value for this event
    Source:    "example.org/example-gateway",
    // This is the event's type
    Type:      "create-event",
    // This is the event's payload
    Payload:   "Dolly",
  }
```

We've filled in the core fields needed for any Brigade event, `Source` and
`Type`. As a bonus, we're also adding a `Payload`. However, that's just the
start of what a Brigade Event can contain. Other notable fields worth
researching are:

- `ProjectID`: When supplied, the event will _only_ be eligible for receipt by
  a specific project.

- `Qualifiers`: A list of qualifier values. For a project to receive an event,
  the qualifiers on a project's event subscription must exactly match the
  qualifiers on the event (in addition to matching source and type).

- `Labels`: A list of labels. Projects can choose to utilize these for
  filtering purposes. In contrast to qualifiers, a project's event
  subscription does not need to match an event's labels in order to receive it.
  Labels, however, can be used to narrow an event subscription by optionally
  selecting only events that are labeled in a particular way.

- `ShortTitle`: A short title for the event.

- `LongTitle`: A longer, more descriptive title for the event.

- `SourceState`: A key/value map representing event state that can be persisted
  by the Brigade API server so that gateways can track event handling progress
  and perform other actions, such as updating upstream services.

- `Summary`: A free-form string field that may be populated by the Worker that
  handles the event. For example, specific details around the processing of an
  event can provide further context to end consumers after the Worker finishes.

### Subscribing a project to events from the example gateway

In order to utilize events from the example gateway, we'll need a Brigade
project that subscribes to the corresponding event source
(`example.org/example-gateway`) and event type (`hello`). We'll also
define an event handler that handles these events and utilizes the attached
payload.

Here's the project definition file.  Note the `spec.eventSubscriptions` section
and the default `brigade.ts` script which contains our event handler:

```yaml
apiVersion: brigade.sh/v2
kind: Project
metadata:
  id: example-gateway-project
description: |-
  An example project that subscribes to events from an example gateway
spec:
  eventSubscriptions:
  - source: example.org/example-gateway
    types:
      - hello
  workerTemplate:
    logLevel: DEBUG
    defaultConfigFiles:
      brigade.ts: |
        import { events } from "@brigadecore/brigadier"

        events.on("example.org/example-gateway", "hello", async event => {
          console.log("Hello, " + event.payload + "!")
        })

        events.process()
```

We can save this to `project.yaml` and create it in Brigade via the following
command:

```shell
$ brig project create --file project.yaml
```

### Running the gateway

Now that we have a project subscribing to events from this gateway, we're ready
to run our program! We export the values required by the gateway and then run
it:

```shell
$ export APISERVER_ADDRESS=<Brigade API server address>

$ export API_TOKEN=<Brigade service account token from above>

$ go run main.go
Event created with ID 46a40cff-0689-466a-9cab-05f4bb9ef9f1
```

Finally, we can inspect the logs to verify the event was processed by the
worker successfully and that the event payload came through:

```shell
$ brig event logs --id 46a40cff-0689-466a-9cab-05f4bb9ef9f1

2021-08-13T22:10:12.726Z INFO: brigade-worker version: 0d7546a
2021-08-13T22:10:12.732Z DEBUG: writing default brigade.ts to /var/vcs/.brigade/brigade.ts
2021-08-13T22:10:12.733Z DEBUG: using npm as the package manager
2021-08-13T22:10:12.733Z DEBUG: path /var/vcs/.brigade/node_modules/@brigadecore does not exist; creating it
2021-08-13T22:10:12.734Z DEBUG: polyfilling @brigadecore/brigadier with /var/brigade-worker/brigadier-polyfill
2021-08-13T22:10:12.734Z DEBUG: compiling brigade.ts with flags --target ES6 --module commonjs --esModuleInterop
2021-08-13T22:10:16.433Z DEBUG: running node brigade.js
Hello, Dolly!
```

### Wrapping up

Hopefully this brief guide showing a sample gateway written using Brigade's Go
SDK was helpful. All of the sample code can be found in the
[examples/gateways/example-gateway] directory.

We look forward to seeing the Brigade Gateway ecosystem expand with
contributions from readers like you!

[examples/gateways]: https://github.com/brigadecore/brigade/tree/main/examples/gateways/example-gateway