# Notification System: Executive Overview

## The Goal
The Status Dashboard Notification System is designed to keep users, stakeholders, and engineering teams informed about system incidents and planned maintenance in real-time. It delivers targeted, reliable alerts to the right people at the right time.

## Key Principles

1. **Self-Service for Users** 
   Authenticated users can subscribe themselves to updates for specific components. No administrator intervention is required to start receiving basic email alerts.

2. **Granular Control**
   Subscribers have full control over the "noise level". They can choose to be notified only for major milestones (e.g., incident creation and resolution) or opt-in to receive every single status update and free-form message.

3. **Advanced Routing for Operations** 
   Administrators can define powerful routing policies. For example, critical alerts can be instantly routed to an On-Call MS Teams channel, while minor updates are batched and sent via email.

4. **Security & Privacy First**
   - **Double Opt-In:** All email subscriptions require confirmation via an emailed link to prevent spam and abuse.
   - **Secure Unsubscribe:** One-click unsubscribe links are cryptographically secured, ensuring malicious actors cannot unsubscribe users.

---

## Delivery Roadmap

### Phase 1: Email Notifications (Self-Service)
The first phase focuses on delivering a robust, user-facing email alert system.
* **Subscription Management:** Users can subscribe to specific components or all components.
* **Automated Emails:** Clean, branded emails are automatically dispatched for:
  * Incidents (Created, Status Transitions, Resolved)
  * Planned Maintenance (Scheduled, Started, Completed)
* **Zero Data Loss:** Built on a highly reliable architecture ("Transactional Outbox") that guarantees no alerts are lost, even if the server crashes unexpectedly.

### Phase 2: MS Teams & Smart Routing (Enterprise Ops)
The second phase introduces enterprise-grade routing capabilities designed for internal operations and on-call teams.
* **MS Teams Integration:** Push alerts directly to MS Teams channels via webhooks.
* **Label-Based Routing:** Route alerts based on incident severity, component, or custom labels (e.g., "Route critical database incidents to the DB Ops Team").
* **Smart Grouping:** Prevents "alert fatigue" by batching multiple rapid events into a single consolidated notification.
* **Mute Timings:** Automatically suppress non-critical alerts during weekends, holidays, or known maintenance windows.

---

## The User Experience

### What the Subscriber Sees
1. **Sign Up:** The authenticated user selects the components they care about and subscribes via the API.
2. **Confirm:** They receive a "Please confirm your subscription" email and click the activation link.
3. **Stay Informed:** When an incident occurs, they receive a structured email containing the event title, severity, current status, and a direct link to the dashboard.
4. **Opt-Out:** If they no longer wish to receive updates, a secure one-click unsubscribe link is available in the footer of every email.

### What the Admin Can Do (via API)
* Manage notification policies (create, update, deactivate routing rules).
* Connect new contact points (e.g., an MS Teams webhook endpoint).
* Query delivery logs to audit what alerts were sent and when.

---

## Why This Approach?

* **Delivers Value Quickly:** Phase 1 immediately solves the problem of keeping end-users informed, while Phase 2 tackles complex internal operational needs.
* **Highly Reliable:** We don't drop alerts. Our asynchronous architecture ensures that the main dashboard remains fast while guaranteeing notification delivery.
* **Future-Proof:** The plugin-based design means adding support for additional channels (e.g., SMS) in the future will be straightforward and require minimal architectural changes.
