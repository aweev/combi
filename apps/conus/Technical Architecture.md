# CONUS

## Technical Architecture + Data Model

**Document Status:** Technical Foundation
**Product Phase:** CONUS Zero → Production Foundation
**Preceding Documents:** Constitution, Operating Bible, CONUS Zero Blueprint, Product Strategy, Information Architecture, UX Blueprint, Business & Economic Model, PRD
**Purpose:** Define the technical architecture, application boundaries, core data model, event model, permissions, infrastructure, and engineering principles required to build CONUS.

---

# 01. Technical Mandate

CONUS should be built as a **single coherent system**, not as a collection of disconnected features.

The technical architecture must support the core CONUS loop:

> **DISCOVER → EXPERIENCE → PARTICIPATE → SIGNAL → RECOGNIZE → CREATE OPPORTUNITY → REPEAT**

The architecture must therefore make it easy to:

* publish experiences
* connect content to participation
* record participation
* calculate results
* generate rankings
* recognize people
* manage rewards
* connect creators and founders to opportunities
* connect sponsors to experiences
* measure behavior
* operate local communities
* preserve historical records
* reuse successful formats
* evolve the product without rebuilding the platform

The technical objective is not maximum technical sophistication.

It is:

> **Build the smallest architecture capable of supporting the CONUS system correctly, then expand only when evidence requires it.**

---

# 02. Architectural Principles

## 02.1 Modular Monolith First

CONUS Zero should use a **modular monolith**.

One deployable application can contain clearly separated domains.

Recommended structure:

```text
CONUS Application
│
├── Identity
├── Profiles
├── Locations
├── Content
├── Shows
├── Participation
├── Quizzes
├── Polls
├── Challenges
├── Competitions
├── Results
├── Leaderboards
├── Recognition
├── Rewards
├── Creators
├── Founders
├── Pitches
├── Opportunities
├── Sponsors
├── Campaigns
├── Events
├── Notifications
├── Search
├── Sharing
├── Analytics
└── Administration
```

These modules should have explicit boundaries even though they initially live inside one application.

Do not begin with microservices.

---

# 03. Why a Modular Monolith

CONUS Zero needs rapid experimentation.

The team will still be discovering:

* which participation formats work
* how rankings should behave
* which rewards matter
* how local identity affects participation
* how creators enter the system
* what sponsors actually buy
* how much operational work each format requires

Premature microservices would add:

* deployment complexity
* distributed transactions
* network failure modes
* service monitoring
* infrastructure overhead
* duplicated models
* slower experimentation

The first architectural goal is **clarity of domain boundaries**, not service separation.

A module can become a service later if evidence justifies it.

---

# 04. High-Level System

```text
                    CONUS USERS
                         │
             ┌───────────┴───────────┐
             │                       │
          Mobile Web               PWA
             │                       │
             └───────────┬───────────┘
                         │
                    CDN / Edge
                         │
                  Web Application
                         │
                  Application API
                         │
        ┌────────────────┼─────────────────┐
        │                │                 │
     PostgreSQL         Redis          Object Storage
        │                │                 │
        │                │                 │
        └────────────────┼─────────────────┘
                         │
                 Background Jobs
                         │
        ┌────────────────┼─────────────────┐
        │                │                 │
     Analytics       Notifications     Payments
        │                │                 │
        └────────────────┴─────────────────┘
                         │
                  Admin Command Center
```

---

# 05. Recommended Technology Direction

The exact vendor choices can be finalized during implementation, but the architecture should follow this direction.

## Frontend

Recommended:

* Next.js
* React
* TypeScript
* responsive mobile-first UI
* PWA capabilities
* server-side rendering where useful
* client-side interaction where necessary

The web application should be the primary product.

Do not require a native mobile application for CONUS Zero.

---

# 06. API Layer

The application should expose a structured API between the interface and domain logic.

Possible structure:

```text
/api
    /auth
    /users
    /profiles
    /content
    /shows
    /episodes
    /topics
    /quizzes
    /polls
    /challenges
    /competitions
    /results
    /leaderboards
    /recognition
    /rewards
    /creators
    /founders
    /pitches
    /opportunities
    /sponsors
    /campaigns
    /events
    /notifications
    /search
    /analytics
    /admin
```

The API should expose domain operations rather than raw database tables.

For example:

```text
POST /quizzes/{id}/start
POST /quizzes/{id}/answers
POST /quizzes/{id}/complete
GET  /quizzes/{id}/result
```

rather than allowing clients to manipulate participation records directly.

---

# 07. Database

## Primary Database

Use:

**PostgreSQL**

PostgreSQL should be the authoritative source of truth for transactional product data.

It should store:

* users
* profiles
* content
* participation
* results
* competitions
* rewards
* recognition
* creators
* founders
* sponsors
* campaigns
* events
* opportunities
* subscriptions
* follows
* locations
* permissions
* operational state

---

# 08. Redis

Redis should be used selectively for:

* caching
* sessions where appropriate
* rate limiting
* temporary participation state
* leaderboard acceleration where necessary
* queues/job coordination
* short-lived counters
* anti-abuse controls

Redis should **not** become the authoritative database.

If Redis disappears, CONUS should be able to reconstruct important state from PostgreSQL.

---

# 09. Object Storage

Media should not live directly inside PostgreSQL.

Use object storage for:

* videos
* images
* creator submissions
* Founder application media
* pitch videos
* event media
* thumbnails
* documents
* promotional assets

Database records should store:

```text
media_id
storage_key
media_type
mime_type
size
duration
width
height
status
created_at
```

---

# 10. Media Delivery

Media should be delivered through a CDN.

The media pipeline should eventually support:

```text
Upload
   ↓
Validation
   ↓
Processing
   ↓
Compression
   ↓
Thumbnail Generation
   ↓
Transcoding
   ↓
CDN
   ↓
User
```

For CONUS Zero, do not build a custom video-processing infrastructure unless necessary.

Use managed infrastructure where possible.

---

# 11. Core Domain Architecture

The system can be understood as six major technical layers.

```text
LAYER 1 — IDENTITY
Users
Profiles
Authentication
Permissions

LAYER 2 — EXPERIENCE
Topics
Stories
Shows
Episodes
Events
Creators
Founders

LAYER 3 — PARTICIPATION
Quizzes
Polls
Challenges
Competitions
Votes
Submissions

LAYER 4 — SIGNAL
Results
Scores
Ranks
Points
Follows
Shares
Views
Participation Events

LAYER 5 — RECOGNITION
Badges
Fan of the Week
Star of the Week
Rising of the Week
Champions
Winners

LAYER 6 — OPPORTUNITY
Rewards
Opportunities
Creator development
Founder support
Sponsorship
Production
Events
```

---

# 12. Core Data Philosophy

The database should represent **relationships**, not isolated pages.

For example:

```text
Creator
   ↓
Show
   ↓
Episode
   ↓
Topic
   ↓
Quiz
   ↓
Participation
   ↓
Result
   ↓
Leaderboard
   ↓
Recognition
   ↓
Opportunity
```

A single piece of content may therefore create multiple downstream experiences.

This is fundamental to CONUS.

---

# 13. Entity Model

The initial core entity set is:

```text
User
Profile
Location

Topic
Story
Show
Season
Episode
Media

Creator
Founder
Pitch

Quiz
Question
Poll
Challenge
ChallengeSubmission
Competition
Vote

Participation
Result
Leaderboard
LeaderboardEntry

Recognition
Badge

Reward
RewardEligibility
RewardFulfillment

Sponsor
Campaign
SponsoredExperience

Event

Opportunity

Subscription

Follow
Share
Notification

ParticipationEvent
AuditLog
ModerationCase
```

---

# 14. User

The `User` represents the authenticated account.

```text
User
----
id
email
phone
auth_provider
auth_provider_id
status
created_at
updated_at
last_active_at
```

Possible statuses:

```text
active
suspended
pending
deleted
```

The user record should contain only identity/account information.

Profile information belongs in `Profile`.

---

# 15. Profile

```text
Profile
-------
id
user_id
display_name
username
avatar_media_id
bio
location_id
visibility
created_at
updated_at
```

Additional derived information should generally not be manually stored here.

For example:

* current rank
* points
* streak
* recognition count

should be calculated or maintained by dedicated systems.

---

# 16. Location

Locations are first-class entities.

```text
Location
--------
id
name
slug
type
parent_id
country_code
region_code
status
```

Location hierarchy:

```text
Country
   ↓
Region
   ↓
City / Community
```

Example:

```text
Cameroon
   └── South-West
         └── Kumba
```

A location can therefore participate in:

* leaderboards
* stories
* creators
* founders
* events
* challenges
* recognition
* sponsors

---

# 17. Topic

Topics are one of the core objects of CONUS.

```text
Topic
-----
id
title
slug
description
media_id
status
start_at
end_at
created_by
created_at
updated_at
```

A topic can connect to:

```text
Story
Quiz
Poll
Challenge
Sponsor
Reward
Campaign
Event
```

---

# 18. Content Model

CONUS should use a flexible content structure.

Base:

```text
Content
-------
id
type
title
slug
summary
body
status
author_id
published_at
archived_at
created_at
updated_at
```

Content types may include:

```text
story
article
video
short
quote
announcement
profile_story
event_story
```

Specialized content should reference the base content object rather than duplicating common fields.

---

# 19. Media

```text
Media
-----
id
type
storage_key
url
mime_type
size
duration
width
height
thumbnail_key
processing_status
created_by
created_at
```

Types:

```text
image
video
audio
document
```

---

# 20. Shows

A show represents a recurring entertainment property.

```text
Show
----
id
title
slug
description
cover_media_id
status
creator_id
created_at
updated_at
```

Relationship:

```text
Show
 └── Season
      └── Episode
```

---

# 21. Season

```text
Season
------
id
show_id
number
title
description
status
start_at
end_at
created_at
```

A season creates a temporal container for:

* episodes
* competitions
* recognition
* events
* finale
* archive

---

# 22. Episode

```text
Episode
-------
id
season_id
number
title
description
media_id
duration
status
published_at
created_at
```

An episode may connect to:

```text
Topic
Quiz
Poll
Challenge
Sponsor
Reward
Event
```

---

# 23. Creator

A creator is a first-class participant in the CONUS system.

```text
Creator
-------
id
user_id
display_name
bio
location_id
status
verification_status
created_at
updated_at
```

A creator can have:

```text
Pitches
Shows
Episodes
Stories
Challenges
Events
Recognitions
Opportunities
```

---

# 24. Pitch

```text
Pitch
-----
id
creator_id
title
concept
format
target_audience
rationale
pitch_media_id
supporting_media
status
submitted_at
reviewed_at
reviewed_by
created_at
updated_at
```

States:

```text
draft
submitted
under_review
shortlisted
pilot
accepted
declined
archived
```

---

# 25. Founder

Founders are represented independently from creators because their opportunity journey is different.

```text
Founder
-------
id
user_id
name
location_id
venture_name
problem
solution
stage
pitch
supporting_media
verification_status
status
created_at
updated_at
```

Founder states:

```text
application
verification
profile
competition
finalist
final
winner
alumni
```

---

# 26. Quiz

```text
Quiz
----
id
title
description
topic_id
content_id
time_limit
max_attempts
scoring_type
status
opens_at
closes_at
created_at
updated_at
```

A quiz can be connected to:

```text
Daily CONUS
Episode
Topic
Challenge
Competition
Sponsor
Reward
```

---

# 27. Question

```text
Question
--------
id
quiz_id
position
question_text
type
points
time_limit
created_at
```

Question types:

```text
single_choice
multiple_choice
true_false
```

---

# 28. Answer Option

```text
AnswerOption
------------
id
question_id
position
label
is_correct
created_at
```

Correct answers must never be exposed to the client before completion.

---

# 29. Quiz Participation

Quiz participation should be represented as an explicit record.

```text
QuizAttempt
-----------
id
quiz_id
user_id
started_at
completed_at
status
score
percentage
duration
attempt_number
```

Possible states:

```text
started
completed
abandoned
invalid
```

Individual answers:

```text
QuizAnswer
----------
id
attempt_id
question_id
answer_option_id
answered_at
response_time
```

---

# 30. Poll

```text
Poll
----
id
question
topic_id
content_id
opens_at
closes_at
max_votes_per_user
result_visibility
status
```

Poll options:

```text
PollOption
----------
id
poll_id
label
position
```

Votes:

```text
PollVote
--------
id
poll_id
poll_option_id
user_id
created_at
```

Unique constraints should prevent duplicate votes where required.

---

# 31. Challenge

```text
Challenge
---------
id
title
description
rules
eligibility
submission_type
judging_method
start_at
deadline
results_at
status
creator_id
sponsor_id
reward_id
```

States:

```text
draft
scheduled
open
closing
closed
judging
results
archived
```

---

# 32. Challenge Submission

```text
ChallengeSubmission
-------------------
id
challenge_id
user_id
media_id
description
submitted_at
status
score
judge_notes
```

Potential statuses:

```text
submitted
under_review
shortlisted
finalist
winner
rejected
disqualified
```

---

# 33. Competition

Competition is the container for a competitive experience.

```text
Competition
-----------
id
title
description
type
rules
eligibility
start_at
end_at
status
reward_id
sponsor_id
```

Competition types may include:

```text
quiz
challenge
voting
mixed
```

States:

```text
upcoming
live
closed
verifying
finalized
archived
```

---

# 34. Vote

Votes must be auditable.

```text
Vote
----
id
competition_id
voter_user_id
target_type
target_id
created_at
metadata
```

Possible metadata:

```text
ip_hash
device_hash
session_id
source
```

Sensitive information should be minimized and retained only as needed for abuse prevention.

---

# 35. Results

Results are first-class product objects.

```text
Result
------
id
user_id
source_type
source_id
score
percentage
rank
local_rank
points_earned
streak_value
eligibility_status
created_at
```

A result may originate from:

```text
Quiz
Competition
Challenge
Founder competition
Daily CONUS
```

Results should be shareable.

---

# 36. Leaderboards

Leaderboards should be generated from underlying participation/results rather than treated as manually edited content.

```text
Leaderboard
-----------
id
type
scope
location_id
source_type
source_id
period_type
period_start
period_end
status
```

Types:

```text
national
regional
local
competition
challenge
```

Periods:

```text
day
week
month
season
all_time
```

---

# 37. Leaderboard Entry

```text
LeaderboardEntry
----------------
id
leaderboard_id
user_id
rank
score
points
participation_count
created_at
```

The participant's own position should be easily retrievable.

A user should not need to scroll through 500 people just to know:

> “You are #14 in Kumba.”

---

# 38. Recognition

Recognition is separate from competition.

```text
Recognition
-----------
id
type
user_id
location_id
source_type
source_id
title
description
criteria
period_start
period_end
awarded_at
status
```

Recognition types:

```text
fan_of_week
star_of_week
rising_of_week
local_champion
competition_winner
seasonal_recognition
conus_legend
```

---

# 39. Recognition Logic

The three primary weekly recognitions represent different signals:

```text
Fan of the Week
= participation

Star of the Week
= attention

Rising of the Week
= momentum
```

These should not be reduced to the same score.

The calculation engine should allow separate criteria.

---

# 40. Badges

```text
Badge
-----
id
name
description
icon_media_id
criteria_type
criteria_config
status
```

User badges:

```text
UserBadge
---------
id
user_id
badge_id
awarded_at
source_type
source_id
```

Badges are recognition artifacts, not financial assets.

---

# 41. Points

Points are an internal participation/reputation mechanism.

Points should be separate from money.

```text
PointTransaction
----------------
id
user_id
amount
type
source_type
source_id
description
created_at
```

Examples:

```text
quiz_completion
daily_participation
challenge_entry
streak
referral
recognition
community_action
```

A ledger approach is preferable to simply overwriting a user's point balance.

Current balance can be calculated or maintained as a derived value.

---

# 42. Streaks

```text
Streak
------
id
user_id
type
current_count
longest_count
last_activity_at
updated_at
```

Initially, CONUS should keep streak logic simple.

The important product question is:

> Does returning consistently create meaningful behavior?

Not:

> How complicated can the streak algorithm become?

---

# 43. Rewards

```text
Reward
------
id
name
description
type
value
quantity
funding_source
sponsor_id
eligibility_rules
status
created_at
updated_at
```

Reward types:

```text
cash
product
experience
access
recognition
opportunity
```

---

# 44. Reward Fulfillment

```text
RewardFulfillment
-----------------
id
reward_id
user_id
result_id
status
verification_status
delivery_method
delivery_reference
awarded_at
fulfilled_at
notes
```

States:

```text
pending
verified
awarded
fulfilled
failed
cancelled
```

Rewards should never depend solely on the leaderboard.

Winner verification is required before final fulfillment.

---

# 45. Opportunity

Opportunity is one of the most important long-term entities in CONUS.

```text
Opportunity
-----------
id
type
title
description
recipient_user_id
creator_id
founder_id
sponsor_id
source_type
source_id
value
status
created_at
updated_at
```

Types:

```text
funding
production
mentorship
equipment
exposure
event_invitation
collaboration
distribution
sponsorship
employment
partnership
```

Opportunity is where CONUS moves beyond points and rankings.

---

# 46. Sponsor

```text
Sponsor
-------
id
company_name
contact_name
contact_email
contact_phone
industry
location_id
status
created_at
updated_at
```

---

# 47. Campaign

```text
Campaign
--------
id
sponsor_id
name
package_type
objective
budget
start_at
end_at
status
created_at
updated_at
```

Campaign states:

```text
prospect
proposed
approved
contracted
scheduled
active
completed
reported
renewed
cancelled
```

---

# 48. Sponsored Experience

Sponsors should attach to experiences, not merely pages.

```text
SponsoredExperience
-------------------
id
campaign_id
experience_type
experience_id
sponsor_message
placement_rules
reward_id
status
```

Possible experiences:

```text
topic
quiz
challenge
episode
series
event
reward
season
founder program
```

---

# 49. Event

```text
Event
-----
id
name
description
location_id
venue
start_at
end_at
capacity
ticketing_enabled
sponsor_id
status
```

Events may connect to:

```text
Competition
Creator
Founder
Show
Season
Reward
Campaign
```

---

# 50. Subscription

```text
Subscription
------------
id
user_id
plan
provider
provider_reference
status
started_at
expires_at
cancelled_at
```

Initial plans may include:

```text
free
daily
pro
```

The pricing model remains subject to CONUS Zero validation.

---

# 51. Follow

Following should be represented generically.

```text
Follow
------
id
user_id
target_type
target_id
created_at
```

Targets can include:

```text
creator
founder
show
topic
location
event
```

Unique constraint:

```text
(user_id, target_type, target_id)
```

---

# 52. Share

Sharing should generate an explicit event.

```text
Share
-----
id
user_id
target_type
target_id
channel
created_at
```

Channels:

```text
whatsapp
facebook
instagram
x
copy_link
native_share
```

Sharing itself should not automatically equal a successful referral.

Those are different events.

---

# 53. Referral

A referral can be represented through participation events.

```text
Referral
--------
id
referrer_user_id
referred_user_id
source
created_at
qualified_at
```

A referral should only become “qualified” after the referred person completes a meaningful action.

This prevents empty referral inflation.

---

# 54. Notifications

```text
Notification
------------
id
user_id
type
title
body
target_type
target_id
read_at
created_at
```

Examples:

```text
new_episode
competition_open
vote_closing
result_available
recognition
local_rank_change
creator_update
founder_update
reward_status
```

---

# 55. Participation Event Model

CONUS should maintain a unified behavioral event stream.

```text
ParticipationEvent
------------------
id
user_id
event_type
source_type
source_id
metadata
location_id
session_id
created_at
```

Examples:

```text
topic_viewed
story_opened
video_started
video_completed
quiz_started
question_answered
quiz_completed
poll_voted
challenge_entered
challenge_submitted
competition_entered
vote_cast
result_viewed
result_shared
follow_created
reward_claimed
subscription_started
referral_created
```

This event layer becomes the foundation for analytics and behavioral signals.

---

# 56. Product Event vs Domain Record

These should remain separate.

Example:

A user completes a quiz.

Domain records:

```text
QuizAttempt
Result
PointTransaction
```

Behavioral event:

```text
quiz_completed
```

The domain records describe the state.

The event describes what happened.

This distinction becomes extremely important for analytics.

---

# 57. Audit Log

Administrative actions should be auditable.

```text
AuditLog
--------
id
actor_user_id
action
entity_type
entity_id
before_data
after_data
created_at
```

Examples:

```text
published_content
changed_reward
modified_competition
verified_winner
awarded_reward
changed_sponsor_campaign
suspended_user
```

---

# 58. Moderation

```text
ModerationCase
--------------
id
reporter_user_id
target_type
target_id
reason
status
assigned_to
resolution
created_at
resolved_at
```

Possible statuses:

```text
open
under_review
actioned
dismissed
```

---

# 59. Core Relationships

The most important relationship graph is:

```text
USER
 │
 ├── Profile
 │     └── Location
 │
 ├── Follows
 │
 ├── Participates
 │      │
 │      ├── Quiz
 │      ├── Poll
 │      ├── Challenge
 │      └── Competition
 │
 ├── Results
 │
 ├── Points
 │
 ├── Recognition
 │
 ├── Rewards
 │
 └── Opportunities
```

Content graph:

```text
Topic
 ├── Story
 ├── Quiz
 ├── Poll
 ├── Challenge
 ├── Sponsor
 └── Reward
```

Entertainment graph:

```text
Creator
   ↓
Show
   ↓
Season
   ↓
Episode
   ↓
Topic
   ↓
Participation
```

Commercial graph:

```text
Sponsor
   ↓
Campaign
   ↓
Sponsored Experience
   ↓
Topic / Episode / Challenge / Event
   ↓
Audience Participation
   ↓
Signal
   ↓
Campaign Report
```

Opportunity graph:

```text
Participation
      ↓
Signal
      ↓
Recognition
      ↓
Opportunity
      ↓
Creator / Founder / Fan
```

---

# 60. Daily CONUS Data Structure

Daily CONUS should be assembled rather than treated as a unique hard-coded feature.

```text
Daily CONUS
    │
    ├── Topic
    │
    ├── Story / Media
    │
    ├── Quiz
    │
    ├── Poll
    │
    ├── Challenge
    │
    ├── Sponsor
    │
    ├── Reward
    │
    └── Results
```

This means the same infrastructure can power other properties.

---

# 61. Property Architecture

A CONUS property should be able to assemble reusable primitives.

```text
Property
   │
   ├── Content
   ├── Participation
   ├── Competition
   ├── Reward
   ├── Recognition
   ├── Sponsor
   ├── Event
   └── Analytics
```

Examples:

```text
Daily CONUS
CONUS Short
CONUS Challenge
CONUS Founders
CONUS Pitch
CONUS Live
```

The architecture should avoid creating a completely separate technical system for every property.

---

# 62. State Machines

Important entities must have explicit state transitions.

## Content

```text
DRAFT
  ↓
SCHEDULED
  ↓
PUBLISHED
  ↓
ARCHIVED
```

## Quiz

```text
DRAFT
  ↓
SCHEDULED
  ↓
LIVE
  ↓
CLOSED
  ↓
FINALIZED
```

## Challenge

```text
DRAFT
  ↓
SCHEDULED
  ↓
OPEN
  ↓
CLOSED
  ↓
JUDGING
  ↓
RESULTS
  ↓
ARCHIVED
```

## Competition

```text
UPCOMING
  ↓
LIVE
  ↓
CLOSED
  ↓
VERIFYING
  ↓
FINALIZED
  ↓
ARCHIVED
```

## Pitch

```text
DRAFT
  ↓
SUBMITTED
  ↓
UNDER REVIEW
  ↓
SHORTLISTED
  ↓
PILOT
  ↓
ACCEPTED / DECLINED
```

## Founder

```text
APPLICATION
  ↓
VERIFICATION
  ↓
PROFILE
  ↓
COMPETITION
  ↓
FINALIST
  ↓
FINAL
  ↓
WINNER
  ↓
ALUMNI
```

---

# 63. Transaction Boundaries

Important operations should be atomic.

For example, completing a quiz may involve:

```text
1. Validate attempt
2. Calculate score
3. Create result
4. Award points
5. Update streak
6. Record participation event
7. Determine eligibility
```

These operations should be coordinated so the user cannot receive:

* a result without a completed attempt
* points without participation
* a reward without eligibility
* duplicate completion credit

Use database transactions for state-changing operations where appropriate.

---

# 64. Idempotency

Critical operations must be idempotent.

Examples:

```text
quiz completion
vote submission
reward fulfillment
subscription webhook
payment webhook
recognition award
points transaction
```

If a request is accidentally sent twice, the system should not:

* award double points
* create two results
* cast two votes
* fulfill a reward twice
* charge twice

---

# 65. Anti-Abuse Architecture

CONUS will eventually handle:

* votes
* rewards
* rankings
* referrals
* recognition
* competitions

Therefore abuse prevention must be designed into the system.

Initial controls:

```text
Rate limiting
Duplicate detection
Attempt limits
Vote limits
Referral qualification
Suspicious activity flags
Winner verification
Admin review
Audit logs
```

Do not attempt to build an advanced fraud-detection AI system in Zero.

Build the ability to detect, flag, review, and correct.

---

# 66. Leaderboard Calculation Strategy

For CONUS Zero, leaderboard calculation can initially happen through scheduled jobs and indexed database queries.

Example:

```text
Every 5 minutes
      ↓
Calculate active leaderboard
      ↓
Write ranking snapshot
      ↓
Cache result
```

For high-frequency competitions, Redis can later support real-time ranking.

The authoritative result should remain reconstructable from PostgreSQL.

---

# 67. Recognition Calculation

Recognition should run as a scheduled process.

Example:

```text
Weekly Recognition Job
          ↓
Collect participation signals
          ↓
Collect attention signals
          ↓
Collect momentum signals
          ↓
Apply rules
          ↓
Generate candidates
          ↓
Human review
          ↓
Award recognition
          ↓
Publish
```

Human review remains important.

CONUS should not blindly turn an algorithmic ranking into cultural judgment.

---

# 68. Opportunity Creation

Opportunity should not automatically be awarded solely because a metric is high.

A signal can trigger an opportunity review.

Example:

```text
Rising creator
     ↓
Strong audience response
     ↓
Editorial review
     ↓
Opportunity identified
     ↓
Sponsor / CONUS support
     ↓
Opportunity created
```

This protects the distinction between:

**signal** and **human judgment**.

---

# 69. Search Architecture

Initial search can use PostgreSQL full-text search.

Searchable entities:

```text
People
Creators
Founders
Stories
Shows
Episodes
Topics
Challenges
Competitions
Events
Locations
```

Later, if scale requires it, CONUS can introduce:

* Elasticsearch
* OpenSearch
* Algolia
* another dedicated search engine

Do not introduce it before the product requires it.

---

# 70. Analytics Architecture

Analytics should have two layers.

## Product Analytics

Measures behavior.

```text
Page view
Content view
Video start
Video completion
Quiz start
Quiz completion
Vote
Challenge submission
Result view
Share
Follow
Return
```

## Business Analytics

Measures system value.

```text
Weekly Qualified Participants
Sponsor participation
Campaign engagement
Cost per meaningful attention
Reward cost
Revenue
Contribution margin
Creator pipeline
Opportunity creation
Local participation
Retention
```

---

# 71. Event Naming Convention

Use predictable names.

Format:

```text
<object>_<action>
```

Examples:

```text
topic_viewed
story_opened
video_started
video_completed
quiz_started
question_answered
quiz_completed
poll_voted
challenge_entered
challenge_submitted
competition_entered
vote_cast
result_viewed
result_shared
profile_followed
reward_claimed
reward_fulfilled
```

Avoid inconsistent naming such as:

```text
quizDone
QuizCompleted
completedQuiz
finish_quiz
```

Choose one convention and enforce it.

---

# 72. Analytics Event Payload

A standard event envelope should include:

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
metadata
```

Metadata should remain flexible but controlled.

Do not turn metadata into an unstructured dumping ground.

---

# 73. Privacy

CONUS should follow data minimization.

Collect only what is needed.

Particularly important:

* location
* phone
* email
* identity information
* payment information
* voting information
* reward fulfillment information

Public profile information should be explicitly separated from private account information.

---

# 74. Location Privacy

Location can be used for:

* local leaderboards
* local discovery
* local events
* local recognition
* local content

But CONUS should not expose unnecessary precise location information.

The system should primarily operate at:

```text
Country
Region
City / Community
```

rather than exposing exact user coordinates.

---

# 75. Permissions

Recommended permission layers:

```text
Visitor
Participant
Creator
Founder
Moderator
Editor
Producer
Revenue
Operations
Administrator
Super Admin
```

Permissions should be action-based.

For example:

```text
publish_content
review_pitch
verify_founder
manage_reward
verify_winner
manage_campaign
view_analytics
manage_users
```

Avoid relying only on broad roles.

---

# 76. Admin Architecture

The Editorial Command Center should be a first-class application area.

Primary workflows:

```text
Create Edition
     ↓
Create Topic
     ↓
Attach Content
     ↓
Attach Participation
     ↓
Attach Sponsor
     ↓
Attach Reward
     ↓
Schedule
     ↓
Publish
     ↓
Monitor
     ↓
Close
     ↓
Verify
     ↓
Recognize
     ↓
Report
```

The goal is for normal CONUS operations to happen without engineering intervention.

---

# 77. Admin Dashboard Modules

```text
Dashboard
Content
Topics
Daily CONUS
Shows
Episodes
Creators
Pitches
Founders
Challenges
Competitions
Votes
Results
Leaderboards
Recognition
Rewards
Sponsors
Campaigns
Events
Users
Moderation
Analytics
Settings
Audit Logs
```

---

# 78. Background Jobs

CONUS will need a job system.

Initial jobs:

```text
Publish scheduled content
Close expired competitions
Calculate leaderboards
Calculate recognition
Update streaks
Process notifications
Process reward status
Generate analytics aggregates
Process media
Clean expired sessions
Run abuse checks
```

Jobs should be retryable.

Failed jobs should be observable.

---

# 79. Notifications Architecture

Notifications should be event-driven.

Example:

```text
Competition closes
        ↓
Domain event
        ↓
Notification rule
        ↓
Eligible users
        ↓
Notification queue
        ↓
Delivery
```

Channels may eventually include:

```text
In-app
Email
Push
WhatsApp
SMS
```

CONUS Zero should start with the channels that are operationally practical.

---

# 80. Payments

Payment architecture should isolate the payment provider from CONUS business logic.

```text
CONUS
  ↓
Payment Adapter
  ↓
Payment Provider
```

The application should not assume that a single provider will always be used.

Store:

```text
provider
provider_reference
amount
currency
status
transaction_type
user_id
created_at
```

Payment confirmation should rely on verified provider callbacks/webhooks.

---

# 81. Currency

Because the initial operating context is Cameroon, the system should support:

```text
currency
amount_minor_units
```

rather than storing monetary amounts as floating-point numbers.

Example:

```text
100 FCFA
```

should be represented using an integer monetary value appropriate to the chosen provider/accounting model.

---

# 82. Reward and Payment Separation

A reward should not automatically equal a payment transaction.

For example:

```text
Reward
  ↓
Winner verification
  ↓
Fulfillment decision
  ↓
Payment / delivery
```

This makes it possible to support:

* cash
* products
* tickets
* access
* opportunities
* recognition

without forcing everything through the payment system.

---

# 83. PWA Requirements

The product should support:

```text
Installable experience
Offline shell
Fast navigation
Responsive UI
Mobile-first layouts
Deep linking
Shareable URLs
Push notifications where supported
```

Offline support should focus on shell/navigation rather than trying to make the entire platform offline-first.

---

# 84. Performance Requirements

CONUS Zero should prioritize:

* fast first load
* compressed images
* adaptive video
* minimal JavaScript
* lazy loading
* CDN delivery
* server rendering where useful
* efficient database queries
* pagination
* caching

The product should be designed for environments where bandwidth is limited.

---

# 85. API Performance

Avoid endpoints that return huge objects.

Prefer:

```text
GET /home
GET /topics/{slug}
GET /shows/{slug}
GET /leaderboards/{id}
```

with purpose-built response shapes.

Use pagination for:

```text
feeds
search
leaderboards
participants
submissions
notifications
activity
```

---

# 86. Caching

Cache relatively stable/high-read data:

```text
Published topics
Public profiles
Show metadata
Episode metadata
Leaderboards
Recognition pages
Popular content
```

Do not aggressively cache rapidly changing transactional data.

---

# 87. Database Indexing

Initial indexes should cover:

```text
User.email
User.phone

Profile.username
Profile.location_id

Content.slug
Content.status
Content.published_at

Topic.slug
Topic.status

Show.slug

Creator.user_id
Creator.location_id

Founder.location_id
Founder.status

Quiz.status
Quiz.opens_at
Quiz.closes_at

QuizAttempt.user_id
QuizAttempt.quiz_id

Vote.competition_id
Vote.voter_user_id

Result.user_id
Result.source_type
Result.source_id

Leaderboard.location_id
Leaderboard.source_id
Leaderboard.period_start

Follow.user_id
Follow.target_type
Follow.target_id

ParticipationEvent.user_id
ParticipationEvent.event_type
ParticipationEvent.created_at
```

Composite indexes should be added based on actual query patterns.

---

# 88. Database Constraints

Use database constraints to protect business rules.

Examples:

```text
unique username
unique slug within entity
unique follow relationship
unique vote where applicable
unique active subscription
unique recognition assignment where applicable
foreign key integrity
non-negative reward quantity
valid state transitions
```

Do not rely solely on frontend validation.

---

# 89. Data Integrity Rules

Important rules should exist at the domain/service layer and, where practical, at the database layer.

Examples:

A closed competition cannot accept a new vote.

A finalized competition cannot change its winner without an audited administrative action.

A fulfilled reward cannot return to pending without an explicit reversal process.

A deleted user should not destroy historical participation records required for legitimate reporting.

---

# 90. Historical Data

CONUS has a long-term archive ambition.

Do not design the database as if everything expires.

Historical entities should remain referenceable.

Important historical objects:

```text
Season
Competition
Winner
Recognition
Creator
Founder
Event
Episode
Challenge
Opportunity
Sponsor
```

The archive eventually becomes part of the product.

---

# 91. Five-Year Architecture

The system should support:

```text
Season 1
Season 2
Season 3
Season 4
Season 5
...
```

Historical people can become:

```text
Winner
Alumni
Mentor
Judge
Creator
Sponsor
Partner
```

The data model should therefore preserve relationships across time.

---

# 92. CONUS Legends

“CONUS Legend” should not require a special technical subsystem.

It should be a recognition record connected to:

```text
User
Recognition
Season
Achievement
Historical record
```

This allows the same architecture to support long-term recognition.

---

# 93. Data Ownership

Each domain should have an obvious owner.

```text
Identity
→ Identity module

Content
→ Editorial/Content module

Participation
→ Participation module

Competition
→ Competition module

Rewards
→ Rewards module

Recognition
→ Recognition module

Creators
→ Creator module

Founders
→ Founder module

Sponsors
→ Revenue/Sponsorship module

Analytics
→ Analytics module
```

Modules should not casually modify another module's tables.

Use domain services/events where cross-domain actions are necessary.

---

# 94. Internal Domain Events

CONUS should eventually use internal domain events.

Examples:

```text
QuizCompleted
ChallengeSubmitted
CompetitionFinalized
RecognitionAwarded
RewardFulfilled
CreatorApproved
FounderVerified
CampaignActivated
EpisodePublished
```

A domain event can trigger secondary processes.

Example:

```text
QuizCompleted
     ├── create Result
     ├── award Points
     ├── update Streak
     ├── update Leaderboard
     └── record Analytics Event
```

---

# 95. Domain Event Rule

Events should communicate:

> “Something happened.”

They should not become hidden business logic.

The originating domain remains responsible for its own authoritative state.

---

# 96. API Security

Minimum requirements:

```text
HTTPS
Secure authentication
Session/token protection
Rate limiting
Input validation
Authorization checks
CSRF protection where applicable
Secure file upload handling
Webhook signature verification
Secrets management
Audit logging
```

---

# 97. Authentication

The exact authentication provider should be validated during Zero.

Possible methods:

```text
Phone
Email
Social login
```

The product should optimize for low-friction participation.

Do not force account creation before users understand the value.

---

# 98. Anonymous Participation

CONUS may allow limited anonymous discovery.

Potential model:

```text
Visitor
  ↓
Discover
  ↓
Experience
  ↓
Start participation
  ↓
Account required when value must be retained
```

For example, a user may view a story before registration but need an account to:

* retain results
* appear on a leaderboard
* receive recognition
* claim rewards
* follow creators
* maintain a streak

---

# 99. File Upload Security

User-generated uploads must be validated.

Check:

```text
file type
file size
extension
mime type
malware where available
processing status
ownership
visibility
```

Never trust the filename or client-provided MIME type alone.

---

# 100. Deployment Architecture

Initial production environment:

```text
Internet
   ↓
CDN / Edge
   ↓
Web Application
   ↓
Application Server
   ↓
PostgreSQL
   ↓
Redis
   ↓
Object Storage
```

Background worker:

```text
Application
     ↓
Job Queue
     ↓
Worker
```

Admin can share the same application infrastructure while remaining permission-protected.

---

# 101. Environments

At minimum:

```text
Development
Staging
Production
```

Staging should use non-production credentials and payment systems.

Production data should never be casually copied into development.

---

# 102. Configuration

Environment-specific configuration should be externalized.

Examples:

```text
DATABASE_URL
REDIS_URL
STORAGE_BUCKET
PAYMENT_PROVIDER_KEY
AUTH_SECRET
EMAIL_PROVIDER_KEY
ANALYTICS_KEY
```

Secrets must never be committed to source control.

---

# 103. Observability

Production must provide visibility into:

```text
application errors
API latency
database performance
background jobs
payment failures
media processing
notification failures
authentication failures
traffic
participation activity
```

Use structured logs.

Each important request should have a trace/request ID.

---

# 104. Backup and Recovery

At minimum:

```text
Automated PostgreSQL backups
Point-in-time recovery where supported
Object-storage versioning/backups where appropriate
Recovery procedure
Periodic restore testing
```

A backup that has never been restored is not a proven recovery strategy.

---

# 105. Disaster Recovery Priority

Highest-priority data:

```text
Users
Participation
Results
Votes
Rewards
Payments
Sponsors
Opportunities
Historical recognition
Content
```

Derived caches can be rebuilt.

---

# 106. Data Retention

Retention rules should distinguish:

```text
Account data
Transactional data
Analytics data
Moderation data
Payment data
Historical public data
```

Not every event needs to be stored forever at maximum detail.

But historically meaningful product records should remain available.

---

# 107. Technical Architecture for CONUS Zero

The first build should intentionally be smaller.

```text
WEB / PWA
   │
   ▼
NEXT.JS APPLICATION
   │
   ├── Auth
   ├── Content
   ├── Daily CONUS
   ├── Quiz
   ├── Results
   ├── Leaderboards
   ├── Profiles
   ├── Creator
   ├── Founder
   ├── Recognition
   ├── Rewards
   ├── Sponsor
   └── Admin
   │
   ▼
POSTGRESQL
   │
   ├── REDIS
   ├── OBJECT STORAGE
   └── JOB WORKER
```

This is enough to prove the core system.

---

# 108. CONUS Zero Technical Scope

## MUST BUILD

### Identity

* authentication
* user
* profile
* location

### Discovery

* home
* topics
* stories
* video
* shows
* episodes
* creators
* founders
* local

### Participation

* Daily CONUS
* quizzes
* results
* basic polls
* challenges
* competitions
* voting

### Signal

* points
* leaderboard
* sharing
* follows
* participation events

### Recognition

* Fan of the Week
* Star of the Week
* Rising of the Week
* Local Champion
* competition winners

### Opportunity

* Founder program
* creator pitch
* opportunity records
* rewards

### Commercial

* sponsors
* campaigns
* sponsored experiences

### Operations

* admin
* editorial command center
* moderation
* analytics
* winner verification

---

# 109. CONUS Zero Should NOT Build Yet

Avoid:

```text
Native iOS app
Native Android app
Microservices
Blockchain
Complex recommendation engine
Open creator marketplace
Self-service advertiser marketplace
Advanced AI personalization
Large-scale comments
Complex social graph
International infrastructure
Sophisticated ad auction
Real-time global leaderboard infrastructure
Complex IP licensing system
```

These may eventually become appropriate.

They are not prerequisites for proving CONUS.

---

# 110. Recommended Repository Structure

Example:

```text
conus/
│
├── app/
│   ├── (public)/
│   ├── (auth)/
│   ├── participate/
│   ├── discover/
│   ├── community/
│   ├── create/
│   ├── rewards/
│   ├── profile/
│   └── admin/
│
├── modules/
│   ├── identity/
│   ├── profiles/
│   ├── locations/
│   ├── content/
│   ├── shows/
│   ├── participation/
│   ├── quizzes/
│   ├── polls/
│   ├── challenges/
│   ├── competitions/
│   ├── results/
│   ├── leaderboards/
│   ├── recognition/
│   ├── rewards/
│   ├── creators/
│   ├── founders/
│   ├── pitches/
│   ├── opportunities/
│   ├── sponsors/
│   ├── campaigns/
│   ├── events/
│   ├── notifications/
│   ├── analytics/
│   └── administration/
│
├── components/
├── lib/
├── jobs/
├── db/
├── tests/
└── config/
```

The exact framework structure may differ.

The important principle is domain separation.

---

# 111. Testing Strategy

Testing should exist at multiple levels.

## Unit Tests

Test:

```text
scoring
points
streaks
eligibility
ranking
recognition
reward rules
state transitions
```

## Integration Tests

Test:

```text
quiz completion
vote submission
competition finalization
reward fulfillment
creator submission
Founder competition
sponsor experience
```

## End-to-End Tests

Test critical user journeys:

```text
Visitor → Daily CONUS → Quiz → Result → Share

User → Challenge → Submission → Result

Creator → Pitch → Submission → Status

Founder → Application → Voting → Winner

Admin → Topic → Quiz → Sponsor → Reward → Publish → Close
```

---

# 112. Critical Acceptance Tests

## Daily CONUS

A user can:

```text
open today's experience
understand it
start
participate
receive feedback
see result
see rank
share
discover next action
```

## Competition

The system can:

```text
open competition
accept valid participation
reject invalid participation
close competition
calculate results
verify winners
finalize
publish
```

## Reward

The system can:

```text
define reward
define eligibility
identify winner
verify winner
award reward
track fulfillment
```

## Creator

The system can:

```text
submit pitch
track status
review
shortlist
pilot
publish
measure
```

---

# 113. Technical North Star

The architecture should make this possible:

> **One interesting thing can become many experiences without becoming many disconnected systems.**

Example:

```text
One Creator
     ↓
Pitch
     ↓
Pilot
     ↓
Episode
     ↓
Quiz
     ↓
Challenge
     ↓
Audience Participation
     ↓
Leaderboard
     ↓
Recognition
     ↓
Sponsor
     ↓
Opportunity
     ↓
New Production
```

That is the technical expression of the CONUS flywheel.

---

# 114. Data Model Summary

The most important relationships are:

```text
USER
 ├── PROFILE
 ├── LOCATION
 ├── FOLLOW
 ├── PARTICIPATION
 ├── RESULT
 ├── POINT TRANSACTION
 ├── RECOGNITION
 ├── REWARD FULFILLMENT
 └── OPPORTUNITY

CREATOR
 ├── PITCH
 ├── SHOW
 ├── EPISODE
 ├── CHALLENGE
 └── OPPORTUNITY

FOUNDER
 ├── APPLICATION
 ├── PROFILE
 ├── COMPETITION
 ├── RESULT
 └── OPPORTUNITY

CONTENT
 ├── TOPIC
 ├── STORY
 ├── SHOW
 ├── SEASON
 └── EPISODE

PARTICIPATION
 ├── QUIZ
 ├── POLL
 ├── CHALLENGE
 ├── COMPETITION
 └── VOTE

SIGNAL
 ├── RESULT
 ├── LEADERBOARD
 ├── SHARE
 ├── FOLLOW
 └── PARTICIPATION EVENT

RECOGNITION
 ├── BADGE
 ├── FAN OF WEEK
 ├── STAR OF WEEK
 ├── RISING OF WEEK
 ├── LOCAL CHAMPION
 └── CONUS LEGEND

COMMERCIAL
 ├── SPONSOR
 ├── CAMPAIGN
 ├── SPONSORED EXPERIENCE
 └── REWARD

OPPORTUNITY
 ├── FUNDING
 ├── PRODUCTION
 ├── MENTORSHIP
 ├── EXPOSURE
 ├── DISTRIBUTION
 └── PARTNERSHIP
```

---

# 115. What the Architecture Must Preserve

There are several distinctions that must never be accidentally collapsed.

## Participation ≠ Attention

Someone can participate heavily without being the most watched person.

## Attention ≠ Momentum

Someone can have large attention but little growth.

## Points ≠ Money

Points represent participation/reputation.

Money belongs to the economic system.

## Ranking ≠ Recognition

A leaderboard is a measurement.

Recognition is a cultural/product decision.

## Signal ≠ Opportunity

Signal identifies something worth examining.

Opportunity is the action that follows.

## Sponsor ≠ Editorial Authority

Sponsors can fund experiences.

They should not secretly determine cultural outcomes.

## Location ≠ Exact Geography

Local identity matters.

Unnecessary precise location exposure does not.

---

# 116. The Technical Flywheel

The architecture ultimately supports:

```text
              CREATORS
                  │
                  ▼
               CONTENT
                  │
                  ▼
            EXPERIENCES
                  │
                  ▼
            PARTICIPATION
                  │
                  ▼
               SIGNAL
                  │
          ┌───────┴────────┐
          ▼                ▼
      RECOGNITION        DATA
          │                │
          ▼                ▼
      OPPORTUNITY       SPONSORS
          │                │
          └───────┬────────┘
                  ▼
              FUNDING
                  │
                  ▼
             PRODUCTION
                  │
                  ▼
              CONTENT
```

Underneath the system:

```text
LOCATION
COMMUNITY
REWARDS
HISTORY
```

---

# 117. Architecture Decision Records

As development progresses, major technical decisions should be documented.

Examples:

```text
ADR-001 — Modular monolith
ADR-002 — PostgreSQL as source of truth
ADR-003 — PWA before native apps
ADR-004 — Object storage for media
ADR-005 — Event-based analytics
ADR-006 — Leaderboard calculation strategy
ADR-007 — Payment provider
ADR-008 — Authentication provider
ADR-009 — Search strategy
ADR-010 — Notification architecture
```

This prevents important technical decisions from becoming tribal knowledge.

---

# 118. Build Order

Technical implementation should follow dependency order.

## Phase 1 — Foundation

```text
Project setup
Authentication
Database
Migrations
Environment configuration
CI/CD
Logging
Error handling
```

## Phase 2 — Core Identity

```text
User
Profile
Location
Permissions
Following
```

## Phase 3 — Content

```text
Topic
Story
Media
Show
Season
Episode
Publishing
```

## Phase 4 — Participation

```text
Quiz
Question
Answer
Poll
Challenge
Competition
Vote
```

## Phase 5 — Signal

```text
Result
Points
Streak
Leaderboard
Share
Participation events
```

## Phase 6 — Recognition

```text
Recognition
Badges
Fan of Week
Star of Week
Rising of Week
Local Champion
```

## Phase 7 — Creator / Founder

```text
Creator
Pitch
Founder
Founder competition
Opportunity
```

## Phase 8 — Rewards / Commercial

```text
Reward
Reward fulfillment
Sponsor
Campaign
Sponsored experience
```

## Phase 9 — Operations

```text
Admin
Moderation
Analytics
Reporting
Audit
```

---

# 119. Final Technical Principle

CONUS should not be architected as:

> **a website containing quizzes, videos, profiles, and advertisements.**

It should be architected as:

> **a connected system in which content creates experiences, experiences create participation, participation creates signals, signals create recognition, and recognition creates opportunity.**

The technology exists to make that loop reliable.

The architecture therefore has one primary responsibility:

> **Make the CONUS loop possible, measurable, repeatable, and scalable without making the system unnecessarily complex.**

The first technical milestone is not “build the whole platform.”

It is:

> **Build the smallest reliable version of the CONUS system that can prove people discover, participate, return, share, recognize, and create demand.**

That is CONUS Zero.
