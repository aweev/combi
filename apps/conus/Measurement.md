# CONUS ANALYTICS & MEASUREMENT SPECIFICATION

**Document Type:** Analytics / Measurement / Reporting Specification
**Product:** CONUS
**Release:** CONUS Zero / First Live Season
**Status:** Implementation Specification

---

# 01. PURPOSE

The purpose of this document is to define the measurement system for CONUS.

CONUS should not merely collect analytics.

It should create a reliable understanding of:

* who discovers CONUS
* what they experience
* what they participate in
* what they ignore
* what they return to
* what they share
* what communities respond to
* which creators gain momentum
* which properties generate participation
* how recognition emerges
* what opportunities are created
* what value sponsors receive
* where the product fails
* what the team should change

The measurement system exists to answer one fundamental question:

> **What is actually happening inside the CONUS system?**

---

# 02. MEASUREMENT PRINCIPLE

CONUS should measure behavior, not vanity.

A large number of:

* page views
* followers
* impressions
* registered accounts
* likes

does not automatically mean CONUS is working.

The measurement hierarchy is:

**Attention → Experience → Participation → Signal → Recognition → Opportunity → Return**

The system should therefore prioritize meaningful actions over raw traffic.

---

# 03. NORTH STAR METRIC

## Weekly Qualified Participants

The primary CONUS North Star is:

> **Weekly Qualified Participants (WQP)**

WQP represents the number of distinct people who perform at least one qualifying meaningful participation action within a rolling seven-day measurement period.

A qualifying action may include:

* completing a Daily CONUS quiz
* submitting a valid poll response
* entering a challenge
* submitting a challenge entry
* voting in an eligible competition
* completing another explicitly defined CONUS participation experience

The exact qualification rules must be configured and versioned.

---

# 04. WHY WQP

WQP is intentionally stronger than:

* registered users
* visitors
* page views
* impressions
* followers

A person counts because they **did something**.

This aligns the metric with the CONUS product thesis:

> CONUS turns discovery into participation.

---

# 05. NORTH STAR DEFINITION

For each seven-day period:

**WQP = distinct eligible users with ≥ 1 qualifying participation event**

Exclude:

* test accounts
* internal staff activity
* confirmed fraudulent activity
* invalid/duplicate events
* explicitly excluded administrative actions

Anonymous participation may be counted separately if the product supports it, but should not silently mix with authenticated WQP.

---

# 06. NORTH STAR SUPPORTING METRICS

WQP must always be viewed alongside:

1. Weekly active participants
2. New participants
3. Returning participants
4. Participation completion rate
5. Participation frequency
6. Share rate
7. Follow rate
8. Local participation
9. Creator engagement
10. Founder engagement
11. Recognition engagement
12. Reward engagement
13. Sponsor engagement

WQP tells us whether participation exists.

These metrics help explain why.

---

# 07. CONUS MEASUREMENT FUNNEL

The primary product funnel is:

**Visitor**

↓

**Engaged Visitor**

↓

**Participant**

↓

**Returning Participant**

↓

**Active Fan**

↓

**Recognized Participant**

↓

**Creator / Founder / Champion / Alumni**

Not every person needs to progress through every stage.

The funnel exists to understand movement through the system.

---

# 08. MEASUREMENT LAYERS

CONUS analytics are organized into seven layers.

## Layer 1 — Discovery

Are people finding CONUS?

## Layer 2 — Experience

Are people consuming and understanding what they find?

## Layer 3 — Participation

Are people doing something?

## Layer 4 — Signal

What does their behavior reveal?

## Layer 5 — Recognition

Who and what is gaining momentum?

## Layer 6 — Opportunity

Does participation lead somewhere?

## Layer 7 — Commercial

Does the system create measurable value for partners?

---

# 09. EVENT MODEL

Every important action should produce an analytics event.

Standard event structure:

```text
event_id
event_type
user_id
anonymous_id
session_id
source_type
source_id
location_id
timestamp
properties
```

Additional recommended fields:

```text
platform
device_type
browser
app_version
content_version
campaign_id
experiment_id
referrer
```

---

# 10. EVENT NAMING

Events use:

**object_action**

Examples:

```text
page_viewed
content_opened
video_started
video_completed
quiz_started
quiz_completed
vote_submitted
challenge_entered
challenge_submitted
result_viewed
result_shared
creator_followed
reward_claimed
```

Names must be:

* predictable
* lowercase
* consistent
* versionable
* documented

Avoid multiple names for the same action.

For example, do not create:

```text
quiz_done
quiz_finished
quiz_completed
```

for the same behavior.

Choose one canonical event.

---

# 11. EVENT CATEGORIES

## Discovery Events

* page_viewed
* home_viewed
* discover_viewed
* search_performed
* search_result_opened
* content_opened
* creator_opened
* founder_opened
* location_opened
* topic_opened

## Experience Events

* story_opened
* video_started
* video_completed
* episode_started
* episode_completed
* media_shared

## Participation Events

* quiz_started
* question_answered
* quiz_completed
* poll_started
* poll_voted
* challenge_viewed
* challenge_entered
* challenge_submitted
* competition_entered
* vote_started
* vote_submitted

## Signal Events

* result_generated
* result_viewed
* result_shared
* leaderboard_viewed
* profile_viewed
* creator_followed
* founder_followed
* show_followed
* topic_followed
* location_followed

## Recognition Events

* recognition_viewed
* badge_earned
* recognition_shared
* champion_recognized

## Opportunity Events

* pitch_started
* pitch_submitted
* founder_application_started
* founder_application_submitted
* opportunity_viewed
* opportunity_accepted

## Commercial Events

* sponsor_experience_viewed
* sponsor_experience_started
* sponsor_experience_completed
* sponsor_cta_clicked
* sponsor_reward_viewed

---

# 12. EVENT PROPERTIES

Events should contain the minimum information necessary to understand the action.

Common properties:

```text
object_type
object_id
property_id
topic_id
location_id
campaign_id
sponsor_id
source
position
duration
completion_percentage
```

Participation events may additionally contain:

```text
attempt_id
question_count
score
completion_time
eligibility_status
```

Voting events may contain:

```text
competition_id
target_id
vote_number
eligibility_status
```

---

# 13. EVENT IDENTITY

Every event requires a unique event ID.

This prevents duplicate ingestion.

Important actions should also have idempotency keys.

Especially:

* quiz completion
* vote submission
* reward claim
* payment
* referral qualification

---

# 14. DISCOVERY METRICS

## Visitors

Distinct people entering CONUS during a defined period.

## New Visitors

People entering CONUS for the first known time.

## Returning Visitors

People previously observed who return.

## Discovery Rate

Measures how frequently people move from an entry point into meaningful content.

---

# 15. CONTENT DISCOVERY METRICS

For each content property track:

* impressions
* opens
* unique viewers
* completion
* shares
* follows
* participation starts
* participation completions
* repeat visits

The important distinction is:

> **Exposure is not participation.**

---

# 16. CONTENT ENGAGEMENT

For video:

### Video Start Rate

Video starts / video impressions

### Video Completion Rate

Completed videos / started videos

### Average Watch Time

Total meaningful watch time / video starts

### Completion Distribution

Measure:

* 25%
* 50%
* 75%
* 100%

This helps identify where attention disappears.

---

# 17. STORY ENGAGEMENT

Track:

* story opens
* reading depth where measurable
* time spent
* linked media starts
* linked participation starts
* shares
* follows
* return visits

A story should ideally lead somewhere.

---

# 18. PARTICIPATION METRICS

For each participation property:

* views
* starts
* completions
* completion rate
* unique participants
* repeat participants
* average completion time
* shares
* resulting points
* resulting rankings
* resulting recognition

---

# 19. PARTICIPATION START RATE

**Participation Start Rate = participation starts / eligible experience views**

This measures whether an experience creates enough motivation to act.

---

# 20. PARTICIPATION COMPLETION RATE

**Completion Rate = completed participations / participation starts**

This measures whether the experience itself works.

A low start rate suggests a motivation/discovery problem.

A high start rate with low completion suggests an experience/friction problem.

---

# 21. PARTICIPATION QUALITY

Not all participation is equally valuable.

Track:

* completion
* repeat participation
* sharing
* following
* return behavior
* local activity
* creator discovery
* opportunity interaction

A participant who completes, shares, follows, and returns represents a stronger signal than a one-time click.

---

# 22. DAILY CONUS METRICS

Every Daily CONUS should have a performance record.

Track:

* unique visitors
* content opens
* video starts
* video completion
* quiz starts
* quiz completion
* poll participation
* challenge participation
* result views
* shares
* follows
* local participation
* reward interaction
* return the following day

The daily experience should eventually allow comparison between Daily CONUS topics.

---

# 23. DAILY RETURN RATE

For Daily CONUS:

**Next-Day Return Rate = participants who return within the defined next-day window / participants on the original day**

This is an important habit indicator.

---

# 24. RETENTION

CONUS should measure:

### D1

Returns one day after first meaningful participation.

### D7

Returns within seven days.

### D30

Returns within thirty days.

For longer-term analysis:

* weekly retention
* monthly retention
* seasonal retention

---

# 25. PARTICIPATION FREQUENCY

Measure:

* participations per user
* active participation days
* weekly participation count
* monthly participation count
* consecutive participation days

This identifies whether CONUS is becoming a habit.

---

# 26. ACTIVE FAN

An Active Fan should have an explicit operational definition.

Example:

> A participant who performs a defined minimum number of meaningful participation actions within a rolling period.

The threshold must be documented rather than changed casually.

---

# 27. SHARING METRICS

Track:

* shares
* unique sharers
* shares per participant
* share rate
* shared object
* share destination where measurable
* downstream visits
* downstream participation

---

# 28. VIRAL LOOP

The relevant loop is:

**Participant → Share → New Visitor → Experience → Participant**

Measure:

* shares generated
* visits generated
* participation generated
* qualified participants generated

This is more useful than raw share count.

---

# 29. REFERRAL METRICS

Where referral tracking is implemented:

* referral link generated
* referral clicked
* referred visitor
* referred participant
* qualified referred participant
* referral conversion rate

Referral rewards should only trigger from valid qualified activity.

---

# 30. FOLLOWING METRICS

Track follows for:

* creators
* founders
* shows
* topics
* locations

Measure:

* follow rate
* unfollow rate
* repeat engagement after following
* notifications generated
* notification engagement

A follow should create future discovery value.

---

# 31. LEADERBOARD METRICS

Track:

* leaderboard views
* national views
* regional views
* local views
* competition views
* participant rank
* rank movement
* rank sharing
* participation after leaderboard view

Important:

> Ranking should lead to behavior, not merely display numbers.

---

# 32. LOCAL METRICS

Local is a strategic measurement layer.

Track:

* local participants
* local participation rate
* local return rate
* local shares
* local leaderboard views
* local recognition views
* local creator engagement
* local Founder engagement
* local challenges
* local sponsor activity

---

# 33. LOCAL PARTICIPATION RATE

Compare:

**Local Qualified Participants / Local Reach**

This can help determine whether location meaningfully increases participation.

Do not assume causality from the metric alone.

---

# 34. RECOGNITION METRICS

For every recognition type:

* candidates
* recognized people
* profile views
* recognition views
* shares
* follows generated
* subsequent participation
* opportunity generated

Track separately:

### Fan of the Week

Participation.

### Star of the Week

Attention.

### Rising of the Week

Momentum.

### Local Champion

Local recognition.

---

# 35. CREATOR METRICS

For each creator:

* profile views
* followers
* content views
* completion
* shares
* participation generated
* audience growth
* pitch submissions
* recognition
* opportunities
* repeat audience

Creator momentum should not be reduced to follower count.

---

# 36. CREATOR PIPELINE METRICS

Track:

**Discover → Invite → Pitch → Review → Pilot → Release → Measure → Develop → Opportunity**

At each stage:

* number entering
* number progressing
* conversion rate
* average time
* drop-off

---

# 37. PITCH METRICS

Track:

* pitch starts
* pitch submissions
* completion rate
* shortlisted
* pilots
* accepted
* declined
* archived
* audience engagement
* opportunity generated

---

# 38. FOUNDER METRICS

Track:

* applications
* verified candidates
* profile views
* votes
* voter participation
* finalist progression
* winner engagement
* opportunity generated
* post-competition visibility

---

# 39. FOUNDER COMPETITION FUNNEL

**Application**

↓

**Verification**

↓

**Public Profile**

↓

**Discovery**

↓

**Vote**

↓

**Finalist**

↓

**Final Challenge**

↓

**Winner**

↓

**Opportunity**

Measure conversion at every stage.

---

# 40. REWARD METRICS

Track:

* rewards available
* rewards claimed
* eligible users
* winners
* verification rate
* fulfillment rate
* fulfillment time
* failed fulfillment
* cancelled rewards
* reward-related support requests

---

# 41. REWARD EFFECTIVENESS

Do not ask only:

> “How many people claimed the reward?”

Also measure:

* participation before reward
* participation during reward
* repeat behavior after reward
* sharing
* retention

The goal is to determine whether rewards strengthen the participation system.

---

# 42. OPPORTUNITY METRICS

Track:

* opportunities created
* opportunities viewed
* recipients
* opportunity type
* acceptance
* completion
* creator opportunities
* Founder opportunities
* audience opportunities
* local opportunities

Opportunity is a strategic output of CONUS.

---

# 43. SPONSOR METRICS

Sponsor reporting should measure meaningful attention rather than only impressions.

Track:

### Reach

* unique viewers
* impressions

### Engagement

* opens
* starts
* completions

### Participation

* quiz participation
* challenge participation
* voting
* submissions

### Advocacy

* shares
* follows
* referrals

### Commercial interaction

* sponsor CTA
* offer interaction
* reward interaction
* conversion where measurable

---

# 44. COST PER MEANINGFUL ATTENTION

CONUS should develop toward:

> **Cost per Meaningful Attention**

A possible operational definition:

**Campaign Spend / Qualified Meaningful Attention**

The exact denominator should be defined per campaign type.

It may incorporate:

* completed content
* meaningful participation
* qualified engagement
* repeat engagement

The metric must not be treated as universally identical across every sponsor format.

---

# 45. SPONSOR CAMPAIGN FUNNEL

**Reach**

↓

**Attention**

↓

**Experience**

↓

**Participation**

↓

**Signal**

↓

**Commercial Action**

Track every stage where technically measurable.

---

# 46. SPONSOR REPORT

Each campaign should eventually produce:

### Campaign Summary

* dates
* audience
* property
* sponsor
* objective

### Audience

* reach
* unique participants
* location

### Experience

* views
* completion

### Participation

* starts
* completions
* shares
* votes
* submissions

### Outcome

* follows
* CTA interactions
* reward interactions
* other agreed commercial actions

### Learnings

* what worked
* what did not
* audience behavior
* recommendations for next campaign

---

# 47. COMMERCIAL DATA PRINCIPLE

Do not manufacture metrics to make sponsors happy.

If CONUS cannot measure something reliably, say so.

A smaller trustworthy number is more valuable than a larger questionable number.

---

# 48. CONTENT PERFORMANCE MODEL

Every major content property should receive a performance profile.

Example:

```text
CONTENT PERFORMANCE

Reach
Attention
Completion
Participation
Sharing
Following
Return
Local response
Recognition
Opportunity
Sponsor value
```

This creates a common measurement language across CONUS.

---

# 49. PROPERTY SCORECARD

Each CONUS property should eventually have:

| Dimension     | Measurement             |
| ------------- | ----------------------- |
| Discovery     | Reach / Opens           |
| Attention     | Completion / Watch      |
| Participation | Qualified Participants  |
| Social        | Shares                  |
| Community     | Follows / Local         |
| Recognition   | Recognition generated   |
| Opportunity   | Opportunities generated |
| Commercial    | Sponsor value           |
| Retention     | Return behavior         |

No single score should replace these dimensions.

---

# 50. CONTENT COMPARISON

CONUS should compare properties by behavior, not simply traffic.

Useful comparison:

| Property   | Reach | Completion | Participation | Share | Return |
| ---------- | ----: | ---------: | ------------: | ----: | -----: |
| Property A |     — |          — |             — |     — |      — |
| Property B |     — |          — |             — |     — |      — |
| Property C |     — |          — |             — |     — |      — |

The purpose is learning, not creating a simplistic universal content score.

---

# 51. EXPERIMENT MEASUREMENT

Every meaningful experiment should define:

1. Hypothesis
2. Audience
3. Change
4. Primary metric
5. Secondary metrics
6. Duration
7. Success condition
8. Failure condition
9. Decision

Example:

**Hypothesis**

Adding a local ranking to a challenge increases repeat participation.

**Primary Metric**

Repeat participation.

**Secondary**

* leaderboard views
* shares
* local participation
* next-day return

---

# 52. EXPERIMENT RULE

Do not run experiments without deciding what the result will change.

The purpose of experimentation is:

> **Better decisions.**

Not:

> **More dashboards.**

---

# 53. DATA SEGMENTATION

Metrics should be segmented where meaningful by:

* new vs returning
* location
* property
* creator
* founder
* sponsor
* campaign
* device
* platform
* acquisition source
* participation type
* time period

Avoid excessive segmentation before the underlying sample is meaningful.

---

# 54. LOCATION ANALYTICS

The location hierarchy:

**Country → Region → City/Community**

should be available for relevant analytics.

Examples:

* national participation
* regional participation
* local participation
* local retention
* local sharing
* local recognition

Precise coordinates should not be necessary for normal CONUS analytics.

---

# 55. TIME ARCHITECTURE

Analytics should support:

* hourly
* daily
* weekly
* monthly
* seasonal
* annual
* all-time

Different questions require different periods.

Daily CONUS requires daily analysis.

Seasonal properties require seasonal analysis.

Recognition requires weekly analysis.

Long-term cultural history requires annual/all-time analysis.

---

# 56. SEASON MEASUREMENT

Each CONUS season should have a measurement record.

Track:

* participants
* returning participants
* properties
* creators
* Founders
* challenges
* competitions
* recognition
* rewards
* sponsors
* opportunities
* local activity

The season becomes a historical unit.

---

# 57. ARCHIVE MEASUREMENT

Historical records should remain measurable.

CONUS should eventually be able to answer:

* Who participated?
* Who won?
* Which creators emerged?
* Which Founders were recognized?
* Which properties mattered?
* Which communities participated?
* Which sponsors supported the season?
* What opportunities were created?

This supports the long-term CONUS archive.

---

# 58. DATA QUALITY

Every critical metric should have a known source.

For example:

**Qualified Participants**

← participation table/event stream

**Leaderboard**

← points/competition records

**Reward Fulfillment**

← reward fulfillment records

**Sponsor Participation**

← campaign-linked participation events

No critical dashboard number should depend on an unexplained manual spreadsheet.

---

# 59. DATA RECONCILIATION

For important actions, reconcile:

**Product Record**

against

**Analytics Event**

against

**Aggregate Metric**

Example:

If 1,000 people completed a quiz:

* quiz completion records should reflect approximately 1,000 valid completions
* analytics should reflect the same valid completion population
* dashboard should reflect the same defined population

Differences must be explainable.

---

# 60. DUPLICATE EVENT HANDLING

Analytics ingestion must handle:

* network retries
* browser retries
* duplicate requests
* delayed events

Use:

* unique event IDs
* idempotency
* ingestion deduplication

Do not count technical retries as human behavior.

---

# 61. LATE EVENTS

Events may arrive late.

Analytics pipelines should distinguish:

* event time
* ingestion time

Historical metrics should have a documented reconciliation policy.

---

# 62. DELETED / INVALID DATA

If participation is later determined to be fraudulent:

* preserve the original audit record
* mark the activity invalid
* exclude it from qualified metrics
* recalculate affected rankings where necessary

Do not silently delete historical evidence.

---

# 63. PRIVACY

Collect only data needed for legitimate product and operational purposes.

Analytics should avoid unnecessary personal information.

Use:

* user IDs
* anonymous IDs
* coarse location
* event metadata

Do not collect precise location merely because the analytics system can technically support it.

---

# 64. ADMIN ANALYTICS DASHBOARD

The Editorial Command Center should expose a concise operating dashboard.

### Today

* active participants
* Daily CONUS participants
* completions
* shares
* current competitions
* current winners/recognition
* sponsor activity
* system alerts

---

# 65. PRODUCT DASHBOARD

Core dashboard:

### Acquisition

* visitors
* new users
* acquisition source

### Activation

* engaged visitors
* first participation

### Participation

* WQP
* participation starts
* completion
* repeat participation

### Retention

* D1
* D7
* D30

### Social

* shares
* follows
* referrals

### Community

* local participation
* local retention

---

# 66. CONTENT DASHBOARD

Show:

* top content by reach
* top content by completion
* top content by participation
* top shared content
* fastest-growing content
* creator momentum
* property trends

Avoid turning this into a single simplistic leaderboard.

---

# 67. CREATOR DASHBOARD

Track:

* active creators
* new pitches
* pitches under review
* pilots
* published creators
* creator audience
* creator growth
* creator participation
* opportunities

---

# 68. FOUNDER DASHBOARD

Track:

* applications
* verified candidates
* active competition
* votes
* participation
* finalists
* winner
* opportunities

---

# 69. RECOGNITION DASHBOARD

Track:

* recognition candidates
* Fan of the Week
* Star of the Week
* Rising of the Week
* Local Champions
* recognition views
* recognition shares
* subsequent participation

---

# 70. REWARD DASHBOARD

Track:

* active rewards
* pending verification
* winners
* pending fulfillment
* fulfilled
* failed
* cancelled
* outstanding reward obligations

---

# 71. SPONSOR DASHBOARD

Track:

* active campaigns
* campaign reach
* participation
* completion
* shares
* reward interactions
* sponsor CTA
* campaign status
* reporting status

---

# 72. TECHNICAL HEALTH DASHBOARD

Track:

* uptime
* API latency
* error rate
* database health
* queue failures
* media failures
* authentication failures
* deployment status

Product analytics and technical monitoring should remain connected but distinct.

---

# 73. ALERTING

Create alerts for:

### Technical

* elevated errors
* database failures
* job failures
* storage failures
* authentication failures

### Product

* sudden participation collapse
* result generation failure
* leaderboard update failure

### Trust

* abnormal vote activity
* reward anomalies
* suspicious referral activity

Alerts should indicate when human action is required.

---

# 74. METRIC OWNERSHIP

Each metric must have an owner.

| Metric              | Owner                  |
| ------------------- | ---------------------- |
| WQP                 | Product                |
| Retention           | Product/Growth         |
| Content performance | Editorial              |
| Creator metrics     | Creator/Content        |
| Founder metrics     | Operations/Product     |
| Local metrics       | Community              |
| Reward metrics      | Operations             |
| Sponsor metrics     | Revenue                |
| Technical metrics   | Engineering            |
| Fraud metrics       | Operations/Engineering |

Ownership means someone is responsible for understanding the number, not merely viewing it.

---

# 75. WEEKLY CONUS REVIEW

The weekly review should answer:

### Audience

Are people returning?

### Participation

Are people doing things?

### Content

What are people actually responding to?

### Community

Where is activity emerging?

### Creators

Who is gaining momentum?

### Recognition

What deserves recognition?

### Opportunity

What opportunities emerged?

### Sponsors

What commercial value was created?

### Product

Where is friction?

### Operations

What is difficult to run?

---

# 76. WEEKLY REVIEW FORMAT

Use:

## 1. What happened?

Facts.

## 2. What changed?

Week-over-week movement.

## 3. Why might it have happened?

Evidence-based interpretation.

## 4. What should we investigate?

Unknowns.

## 5. What should we change?

Specific actions.

## 6. What should we stop?

Low-value activity.

## 7. What should we repeat?

Successful patterns.

---

# 77. MONTHLY REVIEW

Monthly analysis should examine:

* retention
* property performance
* creator growth
* local growth
* recognition
* rewards
* sponsor performance
* revenue
* operational cost
* contribution margin where measurable

---

# 78. SEASON REVIEW

At the end of each season:

### Audience

Who participated?

### Culture

What became culturally interesting?

### Content

What properties generated demand?

### Creators

Who emerged?

### Founders

Who gained recognition?

### Community

Which locations became active?

### Commercial

Which sponsor formats worked?

### Opportunity

What changed for participants?

### Operations

What can scale?

---

# 79. THE CONUS MEASUREMENT LOOP

The complete system is:

**Behavior**

↓

**Event**

↓

**Data**

↓

**Metric**

↓

**Insight**

↓

**Decision**

↓

**Experiment**

↓

**New Behavior**

This is the actual purpose of analytics.

---

# 80. MEASUREMENT ANTI-PATTERNS

CONUS should avoid:

### Vanity Metric Obsession

Celebrating traffic without participation.

### Dashboard Theatre

Creating dashboards nobody uses.

### Metric Inflation

Changing definitions to make numbers look better.

### Untracked Manual Changes

Editing critical numbers without audit history.

### Sponsor Vanity Reporting

Presenting unreliable numbers because they sound impressive.

### Engagement Bait

Optimizing only for clicks.

### Algorithm Worship

Treating automated ranking as inherently correct.

### Over-Measurement

Tracking everything without knowing why.

---

# 81. THE MEANING OF A SIGNAL

A signal is not simply an event.

A signal is:

> **Behavior that provides useful evidence about what people value, want, notice, support, or are willing to participate in.**

Examples:

A view = attention.

A completion = stronger attention.

A participation = action.

A share = advocacy/distribution signal.

A follow = future-interest signal.

Repeated participation = habit signal.

Local participation = community signal.

Creator growth = momentum signal.

Voting = preference signal.

Opportunity acceptance = outcome signal.

---

# 82. SIGNAL HIERARCHY

A useful conceptual hierarchy is:

**View**

↓

**Complete**

↓

**Participate**

↓

**Share**

↓

**Return**

↓

**Follow**

↓

**Support**

↓

**Create**

↓

**Opportunity**

The hierarchy is directional, not a universal scoring system.

Different properties may produce different meaningful signals.

---

# 83. REPUTATION MEASUREMENT

CONUS should eventually track reputation separately from points.

Potential reputation signals:

* repeated participation
* recognition
* creator contributions
* community contributions
* successful challenges
* opportunities earned
* long-term activity

Points may reset.

Reputation should compound.

---

# 84. CULTURAL SIGNALS

CONUS should eventually identify:

* rising creators
* rising topics
* rising locations
* rising properties
* rising Founders
* emerging community conversations
* repeated audience interest

These are not automatic truths.

They are signals requiring interpretation.

Human editorial judgment remains important.

---

# 85. AUDIENCE INTELLIGENCE

The measurement system should help CONUS understand:

> What do people care enough about to do something?

Not simply:

> What did people see?

That distinction should influence product decisions.

---

# 86. OPPORTUNITY INTELLIGENCE

The system should identify people and ideas gaining meaningful momentum.

Potential signals:

* rapid audience growth
* repeated participation
* high completion
* high sharing
* local momentum
* creator consistency
* Founder support
* recognition
* cross-property performance

These signals may trigger human review for opportunity.

---

# 87. SPONSOR INTELLIGENCE

Sponsors should eventually learn:

* what audiences engaged with
* what formats worked
* what locations responded
* what experiences generated participation
* what creators attracted attention
* what rewards generated action

This is more valuable than raw impressions alone.

---

# 88. ANALYTICS IMPLEMENTATION ORDER

Build measurement in this order:

### Phase 1

Core events.

### Phase 2

Participation metrics.

### Phase 3

Results and leaderboards.

### Phase 4

Retention and social.

### Phase 5

Creator/Founder metrics.

### Phase 6

Recognition and opportunity.

### Phase 7

Sponsor reporting.

### Phase 8

Advanced intelligence.

Do not build sophisticated intelligence before basic event reliability is proven.

---

# 89. MINIMUM ZERO ANALYTICS

CONUS Zero must reliably measure:

* visitors
* participants
* WQP
* participation starts
* participation completions
* Daily CONUS performance
* results
* rankings
* shares
* follows
* retention
* local participation
* creators
* Founders
* recognition
* rewards
* sponsors
* technical reliability

---

# 90. ANALYTICS ACCEPTANCE TEST

Analytics is ready when:

1. A known test action occurs.
2. The event is generated.
3. The event is correctly attributed.
4. The event reaches the analytics system.
5. The event appears in the expected aggregate.
6. The underlying product record matches.
7. Duplicate events do not inflate results.
8. Invalid activity can be excluded.
9. The metric definition is documented.
10. An owner understands what the metric means.

---

# 91. FINAL CONUS SCORECARD

The ultimate measurement framework is:

| Layer         | Core Question                                   |
| ------------- | ----------------------------------------------- |
| Discovery     | Are people finding us?                          |
| Experience    | Are they actually experiencing something?       |
| Participation | Are they doing something?                       |
| Signal        | What does their behavior tell us?               |
| Recognition   | Who/what is gaining momentum?                   |
| Opportunity   | What becomes possible because of that momentum? |
| Commercial    | Can meaningful value be funded?                 |
| Retention     | Do people come back?                            |

---

# 92. FINAL PRINCIPLE

CONUS should never confuse measurement with meaning.

Analytics tells us:

> **What happened.**

Analysis asks:

> **Why might it have happened?**

Editorial judgment asks:

> **What does it mean?**

Product judgment asks:

> **What should we change?**

And CONUS asks:

> **What opportunity should exist because of what we learned?**

The system therefore becomes:

**ATTENTION**

→ **PARTICIPATION**

→ **SIGNAL**

→ **RECOGNITION**

→ **OPPORTUNITY**

→ **MORE PARTICIPATION**

---

# 93. FINAL OPERATING SENTENCE

> **CONUS measures what people discover, what they experience, what they participate in, what they share, what they return to, what communities respond to, who gains momentum, what opportunities emerge, and what value the system creates—so that every season becomes smarter than the one before it.**
