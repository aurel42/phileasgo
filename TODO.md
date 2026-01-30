# TODO

X switch Wikipedia article retrieval to HTML so we can filter out non-text sections (only leave sections dominated by <p> tags or somethiing)
X dynamically increase view radius based on the number of POIs in the area
X place marker balloons for next POI as soon as the playback for the last POI has finished
X evaluate other strategies for picking geodate tiles in front of the aircraft
/ higher resolution elevation data
- fix dynamic config (reprocess tiles on config change without hammering wikidata)
X frontend: Altitude box: show "valley altitude"
X frontend: show (optional?) AP status
X dynamic visibility boost: only for XL categories
X pick better thumbnails
X better prediction of how long it takes to generate a narrative
- make Phileas a male person (or make it configurable), give him a better and more intense default personality
X add a streaming switch to the UI so it keeps updating in the background
X expand blind spot to 360°
X OBS overlay
- add info about next city or town to script prompt, and find a good way to make use of it
- auto-update edge-tts user agent and key from locally installed Edge
X API counters for all LLMs
- fix beacon balloons hidden by terrain when flying parallel to high terrain
X support multiple screenshot paths (e.g. VR, pancake)
X feature: notification when crossing state or country borders, entering international waters, etc (new narrative type "remark"?)
- handle "area" POIs (like lakes, cities) better, also "length" POIs like rivers and roads
X control the config using transponder codes and ident
X build an frontend app that wraps the browser UI
X marker badges for interesting states (e.g. deferred)
- preplan a whole trip?
- new IDENT/Playback control function: "expand on this"
- change wp prose in prompt from truncated to wrapped
X regression: restore telemetry loop to 1Hz
- ensure we use the correct coordinates for the city/country code (not predicted)
- overlay info panel: allow adaptive font size for the title if it's too long to fit
- POI script: tell the LLM how far away we are and weather the POI is ahead or to the side?
- regression: border announcements no longer working

# flight stage state machine:
- fallback: on_the_ground
- parked: on ground, engine off, gs<1kts
- taxi: on ground, engine on, 5kts>gs>25kts
- hold: on ground, engine on, gs<1kts
- take-off: 1) on ground, gs>40kts, 2) was on-ground, is now airborne, agl<500ft
- fallback: airborne
- climb: airborne, vs>300fpm
- descend: airborne, vs<-300fpm
- cruise: airborne, vs<+/-200 fpm
- landed: was airborne, now on ground
Initialize with the appropriate fallback state, only change state when conditions are met

# Announcement infrastructure:
- Hook into ticker loop, shouldfire once per second

# changes to existing infrastructure
- currently everything that is done generating is immediately queued in playbackQ
- for announcements, we need to be able to generate, hold the generated narration until it triggers, and then start the playback

# Announcements:
- can either fire once per flight (welcome, debrief) or multiple times (border crossing), this is a fixed behavior of the specific announcement
- triggers for example on flight stage state transitions
- can trigger generation in one stage and playback in another stage 
- if the playback is triggered before the generation is started, the announcement is skipped
- if the playback is triggered before the generation is finished, the announcement is queued immediately after generation is finished
- can optionally request an info panel in the frontends, if they do: must provide a title, optionally text and/or image

Example: welcome announcement triggers generation when parked, triggers playback on transition to taxi
Example: debrief announcement triggers generation and playback when landed
Example: border announcement triggers as it does now
Example: screenshot announcement triggers generation and playback when a new screenshot is found

Phase 1) infrastructure
Phase 2) Welcome Announcement
Phase 3) Migration of screenshot, border, debrief
