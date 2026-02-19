# TODO

X switch Wikipedia article retrieval to HTML so we can filter out non-text sections (only leave sections dominated by <p> tags or somethiing)
X dynamically increase view radius based on the number of POIs in the area
X place marker balloons for next POI as soon as the playback for the last POI has finished
X evaluate other strategies for picking geodate tiles in front of the aircraft
/ higher resolution elevation data
X frontend: Altitude box: show "valley altitude"
X frontend: show (optional?) AP status
X dynamic visibility boost: only for XL categories
X pick better thumbnails
X better prediction of how long it takes to generate a narrative
X add a streaming switch to the UI so it keeps updating in the background
X expand blind spot to 360°
X OBS overlay
X API counters for all LLMs
X support multiple screenshot paths (e.g. VR, pancake)
X feature: notification when crossing state or country borders, entering international waters, etc (new narrative type "remark"?)
X control the config using transponder codes and ident
X build an frontend app that wraps the browser UI
X marker badges for interesting states (e.g. deferred)
X change wp prose in prompt from truncated to wrapped
X regression: restore telemetry loop to 1Hz
X overlay info panel: allow adaptive font size for the title if it's too long to fit
X POI script: tell the LLM how far away we are and whether the POI is ahead or to the side?
X debug "patient" mode, and rework urgent/patient/deferral logic
X regression: AP status line no longer visible
X Flight Stage state machine needs tuning: take-off triggers after landing; take-off only when accelerating, landing only when decelerating?
X Flight Stage: refine conditions for stages based on previos stages (e.g. hold only after taxi), remember timestamps for flight stages?!
X rich history: clean up templates for clarity; shorter, more precise language, NO examples
X improve situational/positional awareness of the LLM, describe distance and direction better.
X the label of the range rings sometimes periodically jumps between two range rings, needs hysteresis? Depends on window size, ofc.
X move session persistence to a better place than in a "heartbeat"
X turn trip summary into a log file
X target balloon: reduce altitude as we get closer to it to make it easier to spot objects on the ground
X frontend continues to show paused when sim disconnects
X frontend: click on configuration pill in main tab opens config page on main tab with no way back
X tune visibility boost for distance; should result in more deferrals
X do not request info panel for essay, briefing
X verify: when flying from MN to CN, to we check the chinese wikipedia?
X regression: essay titles are missing, are now only the category
X regression: POIs no longer open info panels
X Trip Themes
X letsgoo announcement frequently way too long
X Morgan Freeman voice broken
X add reinforcement of target language to 2nd pass
X reconsider basing MaxWords on the Wikipedia article length, doesn't work with Chinese articles
X transition from normal to trip replay still buggy
X replay: photo icon for screenshots, markers get different color
X replay: essays: assign icons, use different colors for markers

# Trip Planning
- can we pre-plan a whole trip? Generate a flight plan?

# Trip Summary / Improved Debriefing
X PoC implementation
X remove zoom or fix track de-syncing

# alternative map
X base is Stamen Watercolor
X overlay with labels, can we control the font?
X fog of war: paper reveal? paint map on parchment as it is revealed?
X Victorian compass rose
X new Victorian marker style

- fix dynamic config (reprocess tiles on config change without hammering wikidata)
- make Phileas a male person (or make it configurable), give him a better and more intense default personality
- add info about next city or town to script prompt, and find a good way to make use of it
- auto-update edge-tts user agent and key from locally installed Edge
- fix beacon balloons hidden by terrain when flying parallel to high terrain
- improve handling of "area" POIs (like lakes, cities); also "length" POIs like roads; major rivers are already improved done
- new IDENT/Playback control function: "expand on this"
X more balloon colors
X maps with overlays

regressions:
X starting app with sim state disconnected and no trip replay available: no world map, just water
- Narrator sometimes picks POIs with unexpired LastPlayed
- on collision, POI markers are placed far from their origin, in spite of the small steps in the radial search
X zoom out on "pause" happens, but zooms in again immediately

- random starting locations on the world map
