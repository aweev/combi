# CONUS INFORMATION ARCHITECTURE

## Product structure, navigation, entities, relationships, and information hierarchy

**Version 1.0**

---

# 00 — THE ROLE OF THIS DOCUMENT

The CONUS Product Strategy defines what the product is and what it must accomplish.

This document defines:

> **How the CONUS product is organized.**

It establishes:

* the product hierarchy
* primary navigation
* major product areas
* information architecture
* core entities
* relationships between entities
* content structure
* user/profile structure
* creator structure
* competition structure
* local structure
* reward structure
* sponsor structure
* administrative structure
* URL/deep-link structure
* what belongs inside the core product
* what belongs outside the core product

This is not yet the UX design.

It does not define exact visual layouts.

It defines the **skeleton the UX sits on**.

---

# 01 — THE CONUS INFORMATION ARCHITECTURE PRINCIPLE

CONUS should not be organized around departments.

It should be organized around **what people come to do**.

A user should not need to understand:

* editorial
* advertising
* creator operations
* campaign management
* internal CONUS terminology

to use the product.

The public product should be organized around:

> **Discover → Experience → Participate → Follow → Compete → Create → Recognize → Opportunity**

---

# 02 — THE TOP-LEVEL CONUS STRUCTURE

The product can be organized into seven primary areas:

```text
CONUS
│
├── HOME
│
├── DISCOVER
│
├── PARTICIPATE
│
├── COMMUNITY
│
├── CREATE
│
├── REWARDS
│
└── PROFILE
```

These are the major public-facing information areas.

Supporting systems sit underneath them.

---

# 03 — HOME

## Purpose

Home answers:

> **What's happening on CONUS right now?**

Home is the dynamic entry point.

It should surface the most important current experiences rather than attempt to contain the entire CONUS universe.

### Home can surface:

* Today's CONUS
* featured story
* featured short
* active challenge
* active competition
* rising person
* Fan of the Week
* Star of the Week
* Rising of the Week
* local activity
* creator discovery
* founder discovery
* current rewards
* upcoming live event
* sponsor-supported experience

Home is therefore a **current-state view of CONUS**.

---

# 04 — DISCOVER

## Purpose

Discovery answers:

> **What is worth knowing about?**

Primary discovery categories:

```text
DISCOVER
│
├── Stories
├── Shows / Shorts
├── People
├── Creators
├── Founders
├── Challenges
├── Competitions
├── Topics
├── Trends
├── Local
└── Events
```

Discovery should allow both:

### Editorial discovery

What CONUS wants to surface.

### Behavioral discovery

What audience signals indicate is gaining attention.

---

# 05 — STORIES

Stories are editorial/content objects.

A story may contain:

* headline
* summary
* body
* images
* video
* audio
* related people
* related topics
* related location
* participation action
* sponsor
* publication date

A story can lead to:

* quiz
* poll
* challenge
* discussion
* creator profile
* founder profile
* related story

---

# 06 — SHOWS / SHORTS

CONUS Short and future entertainment properties should exist as structured content collections.

Hierarchy:

```text
SHOW
│
├── Show Profile
│
├── Season
│   ├── Episode
│   ├── Episode
│   └── Episode
│
├── People
├── Related Content
└── Participation
```

For example:

```text
CONUS Short
└── The Last 500
    └── Season 1
        ├── Episode 1
        ├── Episode 2
        ├── Episode 3
        ├── Episode 4
        └── Episode 5
```

An episode can connect directly to:

* question
* poll
* challenge
* creator
* discussion
* next episode

---

# 07 — PEOPLE

People should be discoverable independently of content.

A person may be:

* fan
* creator
* founder
* contestant
* artist
* business owner
* public personality
* competition participant

A person profile can therefore contain multiple roles.

### Example

A user might be:

> Fan + Creator + CONUS Challenge contestant.

The architecture should not require three separate accounts.

---

# 08 — CREATORS

Creator is a specialized public profile.

Hierarchy:

```text
CREATOR
│
├── Profile
├── Bio
├── Location
├── Work
├── Shows
├── Pitches
├── Challenges
├── Achievements
├── Recognition
├── Followers
└── Opportunities
```

Creator content should link back to the creator.

This creates a network:

**content ↔ creator ↔ audience**

---

# 09 — FOUNDERS

Founders are another specialized public profile.

Hierarchy:

```text
FOUNDER
│
├── Profile
├── What I Do
├── What I've Built
├── Story
├── CONUS Founder Season
├── Challenges
├── Votes
├── Recognition
├── Progress
└── Opportunity
```

The Founder profile should remain useful after the competition ends.

---

# 10 — TOPICS

Topics provide contextual organization.

Examples:

* Music
* Film
* Food
* Fashion
* Technology
* Culture
* Business
* Sports
* Education
* Local
* People

Topics connect multiple object types.

A topic can contain:

* stories
* videos
* quizzes
* challenges
* creators
* founders
* events
* businesses

This prevents content from becoming isolated.

---

# 11 — TRENDS

CONUS Trends should surface things gaining meaningful attention.

A trend can be:

* person
* creator
* topic
* story
* challenge
* location
* event
* cultural moment

A trend should have supporting signals.

The architecture should distinguish:

> **Trending**

from:

> **Editor's Pick**

from:

> **Rising**

These are not the same thing.

---

# 12 — PARTICIPATE

Participation is one of the primary product areas.

```text
PARTICIPATE
│
├── Quizzes
├── Polls
├── Challenges
├── Competitions
├── Voting
├── Predictions
└── Submissions
```

Not every participation mechanism must exist in Zero.

The architecture allows them to coexist.

---

# 13 — QUIZZES

A quiz is a structured participation object.

Hierarchy:

```text
QUIZ
│
├── Title
├── Description
├── Topic
├── Questions
├── Answers
├── Rules
├── Timer
├── Eligibility
├── Scoring
├── Results
├── Ranking
└── Reward
```

A quiz can be attached to:

* Daily CONUS
* Story
* Episode
* Sponsor topic
* Event
* Challenge

---

# 14 — POLLS

Polls are lightweight participation.

Structure:

```text
POLL
│
├── Question
├── Options
├── Eligibility
├── Start
├── End
├── Results
└── Related Content
```

Polls should be lower-friction than competitions.

---

# 15 — CHALLENGES

A challenge is a more substantial participation object.

Hierarchy:

```text
CHALLENGE
│
├── Overview
├── Rules
├── Eligibility
├── Brief
├── Participants
├── Submissions
├── Voting
├── Leaderboard
├── Reward
├── Winner
└── Related Content
```

A challenge can belong to:

* CONUS Challenge
* CONUS Short
* CONUS Founders
* Sponsor
* Live event

---

# 16 — COMPETITIONS

Competition is a container for structured competitive experiences.

Hierarchy:

```text
COMPETITION
│
├── Overview
├── Season
├── Stages
├── Participants
├── Rules
├── Voting
├── Scoring
├── Leaderboards
├── Rewards
├── Winners
└── Archive
```

A competition may contain multiple challenges.

---

# 17 — VOTING

Voting should be treated as an independent system that can attach to multiple experiences.

A voting object contains:

* question
* candidates/options
* eligibility
* voting period
* vote limit
* results
* integrity status

Possible relationships:

```text
CHALLENGE → VOTE

FOUNDER → VOTE

CREATOR → VOTE

EVENT → VOTE

CONTENT → VOTE
```

---

# 18 — RESULTS

Results are important shareable objects.

A result can contain:

* score
* percentage
* rank
* local rank
* regional rank
* national rank
* achievement
* badge
* reward
* comparison
* share action

Example:

> **9/10**

> **#7 in Kumba**

> **#214 nationally**

The result itself should have a deep-linkable URL.

---

# 19 — LEADERBOARDS

Leaderboards can exist at several scopes.

### NATIONAL

All eligible participants.

### REGIONAL

Regional participants.

### LOCAL

City/community participants.

### COMPETITION

Participants in a specific competition.

### CHALLENGE

Participants in a specific challenge.

### TIME

Daily / weekly / seasonal.

Architecture:

```text
LEADERBOARD
│
├── Scope
├── Time Period
├── Participants
├── Rankings
├── Score
├── Movement
└── Recognition
```

---

# 20 — COMMUNITY

Community is where participation becomes identity.

```text
COMMUNITY
│
├── Local
├── Leaderboards
├── Recognition
├── Fans
├── Events
└── Conversations / Discussions
```

Community should not automatically mean a generic social feed.

It should be organized around shared participation.

---

# 21 — LOCAL

Local is a first-class architecture layer.

Hierarchy:

```text
LOCAL
│
├── Country
│   ├── Region
│   │   ├── City / Community
│   │   │   ├── People
│   │   │   ├── Creators
│   │   │   ├── Founders
│   │   │   ├── Stories
│   │   │   ├── Challenges
│   │   │   ├── Events
│   │   │   └── Leaderboards
```

The exact geographic hierarchy should be configurable rather than hard-coded.

---

# 22 — LOCAL PROFILE

A local page can answer:

> **What's happening here?**

It can contain:

* local stories
* local creators
* local founders
* local rankings
* local challenges
* local events
* local businesses
* local champions
* local trends

The local page becomes a community discovery surface.

---

# 23 — RECOGNITION

Recognition should have its own architecture.

```text
RECOGNITION
│
├── Fan of the Week
├── Star of the Week
├── Rising of the Week
├── Local Champion
├── Competition Winners
├── Seasonal Recognition
└── CONUS Legends
```

Each recognition record should link to:

* person
* reason
* supporting signals
* date
* location
* property
* season
* related content

This allows recognition to become part of a permanent history.

---

# 24 — FAN OF THE WEEK

Structure:

```text
FAN OF THE WEEK
│
├── Person
├── Period
├── Location
├── Participation Signals
├── Achievement
├── Story/Profile
└── Shareable Recognition
```

---

# 25 — STAR OF THE WEEK

Structure:

```text
STAR OF THE WEEK
│
├── Person
├── Period
├── Property
├── Attention Signals
├── Content
├── Achievement
└── Shareable Recognition
```

---

# 26 — RISING OF THE WEEK

Structure:

```text
RISING OF THE WEEK
│
├── Person / Idea
├── Period
├── Starting Point
├── Current Signal
├── Momentum
├── Supporting Content
└── Recognition
```

The architecture should preserve the underlying momentum data.

---

# 27 — REWARDS

Rewards should be a separate system rather than hard-coded into every competition.

```text
REWARDS
│
├── Available
├── Earned
├── Pending
├── Redeemed
└── History
```

A reward can be associated with:

* quiz
* challenge
* competition
* recognition
* sponsor campaign
* event
* subscription
* referral

---

# 28 — REWARD OBJECT

A reward contains:

* name
* type
* value
* sponsor
* eligibility
* quantity
* start
* end
* claim rules
* verification
* status

Reward types:

* cash
* voucher
* product
* experience
* access
* recognition
* opportunity

---

# 29 — PROFILE

The profile is the user's personal CONUS identity.

```text
PROFILE
│
├── Identity
├── Location
├── Activity
├── Points
├── Streak
├── Rank
├── Wins
├── Badges
├── Recognition
├── Following
├── Participation History
├── Rewards
└── Settings
```

The profile should emphasize:

> **What have you done?**

rather than simply:

> **How many followers do you have?**

---

# 30 — PROFILE IDENTITY

Keep identity minimal at first.

Potential fields:

* display name
* username
* profile image
* location
* bio
* roles
* achievements

Additional identity requirements can be introduced when necessary for:

* rewards
* competitions
* verification
* payments
* safety

---

# 31 — FOLLOWING

Users can follow:

* creators
* founders
* shows
* competitions
* topics
* local communities

Following creates a personalized signal without requiring CONUS to become a generic social network.

---

# 32 — ACTIVITY HISTORY

A user can eventually see:

* quizzes completed
* challenges entered
* votes
* wins
* recognition
* rewards
* creators followed
* shows followed
* events attended

This creates a personal participation archive.

---

# 33 — CREATE

Create is the entry point for people who want to contribute.

```text
CREATE
│
├── Pitch an Idea
├── Become a Creator
├── Apply to Founders
├── Enter a Challenge
└── Submit Content
```

The exact options can change depending on user role.

---

# 34 — CONUS PITCH

Pitch submission structure:

```text
PITCH
│
├── Concept
├── Format
├── Description
├── Audience
├── Sample
├── 60-second Pitch
├── Creator
├── Status
└── Review
```

Possible status:

**Draft**

→ **Submitted**

→ **Reviewing**

→ **Shortlisted**

→ **Pilot**

→ **Selected**

→ **Developing**

→ **Active**

→ **Archived**

---

# 35 — CONUS FOUNDERS

Founder application structure:

```text
FOUNDERS
│
├── Application
├── Verification
├── Candidate Profile
├── Competition
├── Challenges
├── Votes
├── Final
├── Winner
└── Opportunity
```

---

# 36 — CONUS EVENTS

Events should be discoverable independently.

```text
EVENTS
│
├── Upcoming
├── Live
├── Past
└── Archive
```

An event can connect to:

* competition
* sponsor
* creator
* show
* challenge
* ticket/access
* winner
* content

---

# 37 — CONUS LIVE

CONUS Live is a property, while Events are the underlying information structure.

For example:

**CONUS Live**

→ **CONUS Night 01**

→ Event

→ Live Quiz

→ Challenge

→ Creator Performance

→ Founder Final

→ Awards

This distinction allows future event types without rebuilding the architecture.

---

# 38 — SPONSORS

Sponsors should eventually have public-facing entities.

```text
SPONSOR
│
├── Brand Profile
├── Sponsored Experiences
├── Campaigns
├── Rewards
├── Events
└── Archive
```

A sponsor can be connected to multiple experiences.

---

# 39 — SPONSORED EXPERIENCE

Instead of treating sponsorship as an ad object, structure it as an experience relationship.

```text
SPONSOR
      ↓
CAMPAIGN
      ↓
EXPERIENCE
      ↓
AUDIENCE
      ↓
PARTICIPATION
      ↓
MEASUREMENT
```

Examples:

**Brand → Challenge**

**Brand → Reward**

**Brand → Daily CONUS**

**Brand → Series**

**Brand → Event**

---

# 40 — CAMPAIGNS

Campaigns are primarily an internal/commercial object.

Hierarchy:

```text
CAMPAIGN
│
├── Sponsor
├── Objective
├── Experience
├── Start
├── End
├── Audience
├── Creative
├── Reward
├── Budget
├── Metrics
└── Report
```

In Zero, most campaign management can remain internal.

---

# 41 — CONTENT RELATIONSHIPS

The architecture should make content highly interconnected.

For example:

```text
STORY
 ↓
PERSON
 ↓
CREATOR
 ↓
SHOW
 ↓
EPISODE
 ↓
QUIZ
 ↓
RESULT
 ↓
LEADERBOARD
 ↓
RECOGNITION
 ↓
REWARD
```

A single piece of content can therefore become an entry point into many parts of CONUS.

---

# 42 — THE CORE CONUS ENTITY GRAPH

At the center of the architecture:

```text
                 PERSON
                /      \
          CREATOR      FAN
             │           │
             ↓           ↓
          CONTENT ← PARTICIPATION
             │           │
             ↓           ↓
          PROPERTY ← COMPETITION
             │           │
             ↓           ↓
           TOPIC      LEADERBOARD
             │           │
             └─────┬─────┘
                   ↓
              RECOGNITION
                   ↓
                REWARD
                   ↓
              OPPORTUNITY
                   ↓
                SPONSOR
```

This is the underlying product network.

---

# 43 — CORE ENTITY MODEL

The initial conceptual entities are:

### USER

A CONUS account/person.

### PROFILE

Public identity.

### LOCATION

Geographic identity.

### TOPIC

Subject/category.

### CONTENT

A piece of media/editorial material.

### PROPERTY

A CONUS recurring experience.

### SHOW

A structured entertainment property.

### EPISODE

A show installment.

### QUIZ

A structured knowledge/participation experience.

### POLL

A lightweight voting experience.

### CHALLENGE

A competitive participation experience.

### COMPETITION

A structured competitive container.

### SUBMISSION

User/creator entry into an experience.

### VOTE

Audience decision.

### LEADERBOARD

Ranked participant state.

### RECOGNITION

Awarded status.

### REWARD

Economic/experiential benefit.

### EVENT

Physical/digital event.

### CREATOR

Creator-specific profile.

### FOUNDER

Founder-specific profile.

### SPONSOR

Commercial partner.

### CAMPAIGN

Commercial activation.

### OPPORTUNITY

A development/career/support outcome.

### SEASON

A bounded programming period.

---

# 44 — ENTITY RELATIONSHIPS

Important relationships include:

```text
USER
 ├── has → PROFILE
 ├── belongs to → LOCATION
 ├── follows → CREATOR
 ├── follows → TOPIC
 ├── participates in → CHALLENGE
 ├── votes in → COMPETITION
 ├── submits → PITCH
 ├── applies to → FOUNDERS
 ├── earns → REWARD
 └── receives → RECOGNITION
```

```text
CREATOR
 ├── creates → CONTENT
 ├── submits → PITCH
 ├── participates in → CHALLENGE
 ├── appears in → SHOW
 └── receives → RECOGNITION
```

```text
CONTENT
 ├── belongs to → TOPIC
 ├── belongs to → PROPERTY
 ├── features → PERSON
 ├── connects to → QUIZ
 ├── connects to → POLL
 ├── connects to → CHALLENGE
 └── sponsored by → SPONSOR
```

```text
COMPETITION
 ├── contains → CHALLENGES
 ├── contains → VOTING
 ├── produces → LEADERBOARD
 ├── awards → REWARDS
 └── produces → WINNERS
```

---

# 45 — SEASONS

A season is a major organizing object.

A season can contain:

* Daily CONUS
* CONUS Short
* Founders
* Challenges
* Pitch
* Recognition
* Events
* Sponsors
* Winners
* archive

Example:

```text
THE FIRST CONUS SEASON
│
├── Daily CONUS
├── CONUS Short
├── Founders
├── Pitch
├── Challenge
├── Local
├── Recognition
├── CONUS Night 01
└── Season Archive
```

This gives CONUS a narrative structure.

---

# 46 — TIME ARCHITECTURE

CONUS operates across multiple time scales.

### DAILY

Daily CONUS.

### WEEKLY

Fan / Star / Rising.

### MONTHLY

Magazine/editorial cycle.

### SEASONAL

Major competitions and programming.

### ANNUAL

Major events, awards, archives.

### LONG-TERM

Where Are They Now?

Founder classes.

Alumni.

Legends.

The architecture should preserve all of these time dimensions.

---

# 47 — ARCHIVE

Every major CONUS experience should eventually become archival.

```text
ARCHIVE
│
├── Seasons
├── Shows
├── Winners
├── Creators
├── Founders
├── Challenges
├── Events
├── Champions
└── Cultural Moments
```

This is how CONUS builds memory over time.

---

# 48 — DEEP LINKS

Every meaningful object should eventually have its own shareable URL.

Examples:

```text
/conus
/daily
/daily/topic-name
/quiz/quiz-name
/result/result-id
/challenge/challenge-name
/competition/competition-name
/creator/creator-name
/founder/founder-name
/show/show-name
/episode/episode-name
/topic/topic-name
/location/location-name
/event/event-name
/reward/reward-name
/profile/username
```

The exact URL structure can change during implementation.

The principle should not:

> **Every meaningful CONUS object should be directly discoverable and shareable.**

---

# 49 — PUBLIC VS PRIVATE INFORMATION

The architecture should distinguish between public and private data.

### PUBLIC

* display name
* username
* public profile
* achievements
* rankings where applicable
* creator work
* founder profile
* public submissions
* recognition
* public competition results

### PRIVATE

* email
* phone
* payment information
* verification data
* internal fraud signals
* private account settings
* sensitive operational information

The product should expose only what is necessary.

---

# 50 — ADMINISTRATIVE ARCHITECTURE

The public product is only half of CONUS.

The operating system behind it requires an internal structure.

```text
ADMIN
│
├── Dashboard
├── Editorial
├── Content
├── Properties
├── Users
├── Creators
├── Founders
├── Challenges
├── Competitions
├── Voting
├── Leaderboards
├── Rewards
├── Sponsors
├── Campaigns
├── Events
├── Locations
├── Moderation
├── Analytics
└── Settings
```

---

# 51 — EDITORIAL COMMAND CENTER

The editorial workflow should eventually be:

```text
CREATE EDITION
      ↓
CREATE TOPIC
      ↓
ADD CONTENT
      ↓
ADD INTERACTION
      ↓
ADD REWARD
      ↓
ADD SPONSOR
      ↓
SCHEDULE
      ↓
PUBLISH
      ↓
MONITOR
      ↓
CLOSE
      ↓
VERIFY
      ↓
RECOGNIZE
      ↓
ARCHIVE
```

This is the operational backbone.

---

# 52 — ADMIN CONTENT OBJECTS

Editors should be able to manage:

* articles
* videos
* images
* audio
* quizzes
* polls
* challenges
* episodes
* stories
* topics
* people
* properties

The content system should favor reusable objects.

---

# 53 — ADMIN COMPETITION OBJECTS

Operators should manage:

* competition
* stages
* participants
* eligibility
* submissions
* voting
* scoring
* leaderboard
* rewards
* winners
* disputes

---

# 54 — ADMIN REWARD OBJECTS

Operators should manage:

* reward pool
* reward type
* sponsor
* eligibility
* winners
* verification
* payout
* status

The product should make reward state visible internally.

---

# 55 — ADMIN SPONSOR OBJECTS

Operators should manage:

* sponsors
* campaigns
* experiences
* budgets
* placements
* rewards
* performance
* reporting

---

# 56 — MODERATION

Moderation should operate across:

* profiles
* submissions
* comments/discussions if enabled
* creator content
* founder applications
* competition entries
* votes
* reports

Moderation actions can include:

* review
* approve
* reject
* restrict
* remove
* suspend
* escalate

---

# 57 — SEARCH ARCHITECTURE

Search should eventually index:

```text
PEOPLE
CREATORS
FOUNDERS
CONTENT
SHOWS
EPISODES
TOPICS
CHALLENGES
COMPETITIONS
EVENTS
LOCATIONS
BUSINESSES
```

Search results should be grouped intelligently rather than producing one undifferentiated list.

---

# 58 — FILTERING

Useful filters include:

### CONTENT

Topic / format / date / location.

### PEOPLE

Location / role / category.

### CREATORS

Category / location / rising / property.

### CHALLENGES

Active / upcoming / completed.

### EVENTS

Location / date / type.

### LOCAL

Location / topic / property.

Filtering should only become extensive when content volume justifies it.

---

# 59 — NOTIFICATION ARCHITECTURE

Notifications are triggered by meaningful events.

```text
NOTIFICATION
│
├── Type
├── Actor
├── Object
├── Recipient
├── Time
├── Read Status
└── Action
```

Examples:

**Recognition**

"You are Fan of the Week."

**Competition**

"Voting closes in 2 hours."

**Creator**

"Your pitch moved to shortlist."

**Content**

"A new episode is live."

**Rank**

"You moved into the top 10."

---

# 60 — SHARING ARCHITECTURE

Every shareable object should have:

* title
* description
* image
* canonical URL
* platform preview
* share action

High-priority share objects:

* result
* winner
* ranking
* creator
* founder
* challenge
* episode
* recognition

---

# 61 — THE INFORMATION HIERARCHY

At the highest level:

```text
CONUS
│
├── EXPERIENCE
│   ├── Stories
│   ├── Shows
│   ├── Episodes
│   └── Events
│
├── PARTICIPATION
│   ├── Quizzes
│   ├── Polls
│   ├── Challenges
│   ├── Competitions
│   └── Voting
│
├── PEOPLE
│   ├── Fans
│   ├── Creators
│   ├── Founders
│   └── Champions
│
├── COMMUNITY
│   ├── Local
│   ├── Rankings
│   └── Recognition
│
├── OPPORTUNITY
│   ├── Pitch
│   ├── Founders
│   ├── Rewards
│   └── Opportunities
│
└── COMMERCIAL
    ├── Sponsors
    ├── Campaigns
    └── Sponsored Experiences
```

---

# 62 — THE USER'S MENTAL MODEL

The user should not need to think:

> "Where does a challenge object live?"

They should think:

> **"I want to participate."**

The IA should translate their intention into the correct system.

Likewise:

> "I want to find new creators."

→ Discover.

> "I want to compete."

→ Participate.

> "I want to see what's happening near me."

→ Local.

> "I want to submit my idea."

→ Create.

> "I want to see what I've achieved."

→ Profile.

> "I want to see what I can win."

→ Rewards.

---

# 63 — PRIMARY NAVIGATION

A potential initial navigation:

```text
HOME
DISCOVER
PARTICIPATE
COMMUNITY
CREATE
REWARDS
PROFILE
```

This is a strategic structure, not a final UI decision.

The exact mobile navigation should be validated through UX testing.

---

# 64 — SECONDARY NAVIGATION

Within areas:

### DISCOVER

Stories / Shows / People / Topics / Trends / Local.

### PARTICIPATE

Quizzes / Polls / Challenges / Competitions / Voting.

### COMMUNITY

Local / Leaderboards / Recognition / Events.

### CREATE

Pitch / Founders / Creator.

### PROFILE

Activity / Achievements / Following / Rewards / Settings.

---

# 65 — CONUS ZERO IA

For Zero, simplify aggressively.

The initial product can effectively be:

```text
CONUS
│
├── HOME
│
├── DISCOVER
│   ├── Daily CONUS
│   ├── Shorts
│   ├── People
│   └── Local
│
├── PARTICIPATE
│   ├── Quiz
│   ├── Challenge
│   └── Vote
│
├── LEADERBOARD
│
├── CREATE
│   ├── Pitch
│   └── Founders
│
├── REWARDS
│
└── PROFILE
```

Many operational functions remain inside Admin.

---

# 66 — WHAT SHOULD NOT BE TOP-LEVEL IN ZERO

Do not expose every CONUS concept in primary navigation immediately.

For example:

* Trends
* Marketplace
* Sponsor marketplace
* Advanced opportunities
* Alumni
* Legends
* Archive
* Advanced analytics

These can exist underneath the system or remain internal until demand justifies them.

---

# 67 — THE ZERO INFORMATION FLOW

A new visitor should be able to follow:

```text
SHARED LINK
   ↓
CONTENT / EXPERIENCE
   ↓
PARTICIPATE
   ↓
RESULT
   ↓
PROFILE / RANK
   ↓
SHARE
   ↓
DISCOVER MORE
```

This is more important than forcing the user through the home page.

---

# 68 — THE DISCOVERY FLOW

```text
HOME
 ↓
DISCOVER
 ↓
CONTENT
 ↓
PERSON
 ↓
CREATOR
 ↓
SHOW
 ↓
EPISODE
 ↓
PARTICIPATE
 ↓
FOLLOW
```

---

# 69 — THE COMPETITION FLOW

```text
DISCOVER
 ↓
COMPETITION
 ↓
RULES
 ↓
ENTER
 ↓
CHALLENGE
 ↓
SUBMIT
 ↓
VOTE
 ↓
RESULT
 ↓
LEADERBOARD
 ↓
RECOGNITION
 ↓
REWARD
```

---

# 70 — THE CREATOR FLOW

```text
CREATE
 ↓
PITCH
 ↓
SUBMIT
 ↓
STATUS
 ↓
SHORTLIST
 ↓
PILOT
 ↓
AUDIENCE
 ↓
SIGNAL
 ↓
RECOGNITION
 ↓
OPPORTUNITY
```

---

# 71 — THE FOUNDER FLOW

```text
CREATE
 ↓
FOUNDERS
 ↓
APPLY
 ↓
VERIFY
 ↓
PROFILE
 ↓
CHALLENGE
 ↓
AUDIENCE
 ↓
VOTE
 ↓
FINAL
 ↓
WINNER
 ↓
OPPORTUNITY
```

---

# 72 — THE SPONSOR FLOW

The public IA should not expose this initially as a primary audience destination.

Internally:

```text
SPONSOR
 ↓
CAMPAIGN
 ↓
EXPERIENCE
 ↓
AUDIENCE
 ↓
PARTICIPATION
 ↓
RESULT
 ↓
REPORT
```

Eventually:

```text
SPONSOR
 ↓
DISCOVER OPPORTUNITIES
 ↓
SELECT
 ↓
FUND
 ↓
ACTIVATE
 ↓
MEASURE
```

---

# 73 — THE LOCAL FLOW

```text
LOCAL
 ↓
LOCATION
 ↓
WHAT'S HAPPENING
 ↓
LOCAL PEOPLE
 ↓
LOCAL CREATORS
 ↓
LOCAL CHALLENGES
 ↓
LOCAL LEADERBOARD
 ↓
LOCAL RECOGNITION
 ↓
LOCAL EVENT
```

---

# 74 — THE ARCHIVE FLOW

```text
SEASON
 ↓
PROPERTY
 ↓
EVENT
 ↓
PARTICIPANT
 ↓
WINNER
 ↓
RECOGNITION
 ↓
WHERE ARE THEY NOW?
```

This is how the information architecture can accumulate cultural memory rather than constantly replacing the past.

---

# 75 — INFORMATION ARCHITECTURE PRINCIPLES

### PRINCIPLE 1

**Organize around user intent, not internal departments.**

### PRINCIPLE 2

**Every important object should connect to other objects.**

### PRINCIPLE 3

**People are first-class entities.**

### PRINCIPLE 4

**Participation should be attached to content wherever possible.**

### PRINCIPLE 5

**Local is a system, not a filter.**

### PRINCIPLE 6

**Recognition should persist beyond the competition that created it.**

### PRINCIPLE 7

**Every major experience should be shareable.**

### PRINCIPLE 8

**Every important object should eventually be archival.**

### PRINCIPLE 9

**Zero should expose only the complexity users need.**

### PRINCIPLE 10

**The architecture should support future expansion without requiring the Zero product to be complicated.**

---

# 76 — THE CONUS OBJECT RELATIONSHIP

The most important structural relationship is:

```text
PERSON
   ↓
PARTICIPATION
   ↓
EXPERIENCE
   ↓
CONTENT
   ↓
SIGNAL
   ↓
RECOGNITION
   ↓
OPPORTUNITY
```

Around it:

```text
LOCATION
TOPIC
CREATOR
SPONSOR
REWARD
EVENT
SEASON
```

These become connective tissue across the system.

---

# 77 — INFORMATION ARCHITECTURE → UX

This architecture now gives the UX team/product design process the structure needed to answer:

* What is the home?
* What is the navigation?
* What is a content page?
* What is a creator page?
* What is a challenge?
* What is a result?
* What is a leaderboard?
* What is a profile?
* What does local look like?
* How does a user move between objects?

The next document should therefore focus on **experience and interaction**, not more conceptual strategy.

---

# 78 — INFORMATION ARCHITECTURE → PRD

The eventual PRD can derive its requirements from this structure.

For example:

**IA says:**

> A Challenge contains rules, participants, submissions, voting, leaderboard and reward.

**PRD will say:**

> The system shall allow an administrator to create a challenge with rules, define eligibility, configure submission dates, enable voting, assign rewards, and publish results.

That is the correct separation.

---

# 79 — THE CONUS INFORMATION ARCHITECTURE MODEL

The entire product can ultimately be understood as:

```text
                         CONUS
                           │
       ┌───────────────────┼───────────────────┐
       ↓                   ↓                   ↓
   DISCOVER            PARTICIPATE          CREATE
       │                   │                   │
       ↓                   ↓                   ↓
  CONTENT / PEOPLE    QUIZ / VOTE /       PITCH /
  SHOWS / TOPICS      CHALLENGE            FOUNDERS
       │                   │                   │
       └───────────────────┼───────────────────┘
                           ↓
                       COMMUNITY
                           │
                  LOCAL / RANKINGS /
                    RECOGNITION
                           │
                           ↓
                       REWARDS
                           │
                           ↓
                      OPPORTUNITY
                           │
                           ↓
                        SPONSORS
                           │
                           ↓
                        FUNDING
                           │
                           ↓
                    MORE EXPERIENCES
```

---

# 80 — FINAL IA PRINCIPLE

The CONUS information architecture should make one thing possible:

> **Anything interesting in CONUS should lead naturally to something else worth doing.**

A story should lead to a person.

A person should lead to a creator.

A creator should lead to a show.

A show should lead to participation.

Participation should lead to a result.

A result should lead to a ranking.

A ranking should lead to recognition.

Recognition should lead to opportunity.

Opportunity should create another story.

And that story starts the loop again.

**CONUS is therefore not a collection of pages.**

It is a connected information system in which:

**People, content, participation, community, recognition, opportunity, and commercial experiences continuously connect to one another.**

That connectedness is the architecture.
