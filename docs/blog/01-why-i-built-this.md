# Why I Built This

A few years ago I went for an interview and didn't get the job.

The requirement was that the person knew how to write Kubernetes controllers and operators. Several of them. I knew Kubernetes. I knew how to deploy to it, how to operate it, how to think about it. But I had never written an operator from scratch, and that gap showed.

I decided I wasn't going to miss another opportunity because of that.

---

I started learning. I hit a wall — not the architecture, not the concept. The scaffolding. The boilerplate. The sheer amount of code that stood between me and a working operator. What I had assumed was an advanced skill turned out to require expert-level knowledge just to get started. Every tutorial I followed assumed I already knew client-go, informers, schemes, workqueues. Every framework I tried gave me a project structure I didn't fully understand and hundreds of lines of generated code I couldn't easily modify.

The idea was simple. The path to implementing the idea was not.

---

The further I got, the more I started noticing something. The reconcile loop was always the same shape. The informer setup was always the same. The queue depth, the retry logic, the event handling — identical across every operator I read. The business logic — the actual interesting part, the thing that made each operator useful — was a small fraction of the total code. Everything else was scaffolding that existed for its own sake.

I started thinking about that pattern. If the structure is always the same, why is everyone writing it again?

---

I started talking to engineers. Not to validate a product idea. Just because I had questions. What I heard was the same experience repeated in different words.

*"We needed six operators. We built one and ran out of time."*

*"I got the scaffolding running and then saw how much I still had to write."*

*"We just do it manually now."*

And from developers who just wanted to deploy their applications:

*"I just want my app running. I don't want to learn Kubernetes to do it."*

Nobody was describing an edge case. They were describing the default experience.

---

I kept building. I built operators differently each time — sometimes borrowing patterns from the last one, sometimes starting fresh. At some point I had enough examples in front of me that I stopped thinking about them as programs and started thinking about them as **behaviours**. A deployment is a pattern. A service is a pattern. A secret generated once is a pattern. The operator is just the thing that applies those patterns when a CR appears and keeps applying them until the CR is gone.

If the behaviours are patterns, you only have to describe them once.

That insight was the whole thing. You don't write a new reconciler for each CRD. You write a description of what a reconciler for that CRD should do — what resources it creates, when it creates them, what it watches, how it reports health. A runtime reads that description and does the work. Your job is the description, not the implementation.

That is what the Katalog is: a description of operator behaviour. And that is what the runtime is: the single thing that reads Katalogs and runs them.

The first version was still Go. One runtime with a registry of typed reconcilers: the scaffolding was shared, but every behaviour was still a program. A few days later the description became YAML, and the Go stayed, for the logic YAML should not try to express. That is why Inrun still runs Go reconcilers beside declarative ones. It began with them.

---

I called it Orkestra. An orchestra takes distinct instruments and plays them together, and that is how I saw it then: many operators, each described in its own Katalog, one runtime playing them all. The rest of the vocabulary followed the music, and for a while it fit.

It fit the platform engineers I had talked to. It did not answer the developer.

---

## Half an answer

Looking back, I had answered the first three quotes. Platform teams could describe an operator instead of building one. But *"I just want my app running. I don't want to learn Kubernetes to do it"* was still waiting. To use what the platform team declared, that developer still had to write a custom resource, with an `apiVersion`, a `kind` and a `spec`, and apply it with kubectl.

You can see it in how I kept describing the project. *A declarative runtime for Kubernetes operators.* Then *for Kubernetes behaviours.* Then *a declarative control plane.* Then *the missing layer between Kubernetes and the teams who use it.* Six descriptions in two years. Each one was accurate when I wrote it, and each one was outgrown by the next thing I built.

## The gateway

The next thing was the gateway: an HTTP endpoint in front of the cluster that accepted a request in the caller's own words — a form, a pipeline, a Slack command — and built the Kubernetes object itself.

```json
{"target": "app", "repository": "myorg/payments-api", "environment": "staging"}
```

No `apiVersion`. No `kind`. No `spec`. That was the developer's answer. They could say what they wanted, and the platform would turn it into whatever Kubernetes needed. I started calling what they sent an **intent**, and I kept using the word without noticing how often.

## The question that changed it

Then I asked a question I should have asked at the start: *what is a reconciler actually for?*

In most of the operators I had seen, the reconciler's real job was to call logic that already existed somewhere else — a service the team already ran, an API that already knew the rules. Teams were building a second program, with client-go and informers and RBAC, just to call the first one reliably.

So I let the service be the reconciler. The runtime calls one endpoint on a service that is already serving traffic, sends it the intent, and gets back what should exist and what to report. The runtime keeps everything else: the queue, the retries, the status, the cleanup. The service adds one endpoint and gets operator infrastructure without writing an operator.

## 2 October 2026

That was the day the whole loop was in front of me at once.

An intent comes in through the gateway, in the caller's words. Kubernetes stores it and keeps it true. The runtime reconciles it — from a Blueprint when the behaviour can be declared, or by calling your service when the logic already lives there. The result goes back out through the gateway, again in the caller's words. The person who asked never touches Kubernetes. The team who answers never writes an operator. Kubernetes stays in the middle, doing what it is good at.

I had been writing the word *intent* across several posts for months. The project had been an **in**tent **run**ner for a long time. I just hadn't named it that.

So now it is called one: **Inrun**.

---

I built this because I missed an interview. I kept building it because every engineer I spoke to described the same wall. The goal was never to replace operators — they are the right pattern. The goal was to make them reachable: for the platform engineer who shouldn't need a week of scaffolding, and for the developer who just wants their app running.

I used to end this post with *"Declare. Run."* It turns out the thing being run was never the operator. It was the intent.

Declare once. Send an intent. Inrun keeps it true.
