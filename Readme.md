# EVE Online GoSleuth

GoSleuth is an Eve Online Intel tool. Give it a target and it will pull killmails related to that - be it Pilot, Corp, Alliance, or System - and it will display the data in meaningful ways. This is useful for looking for alts/associates of a pilot, or where they tend to hang out, among many other things.

In the grand tradition of eve tools like PyFa being coded in Python and using Py in its name, GoSleuth is coded in Go-lang.

## Yes. This app used AI in its development

I am a coder by profession, a Senior DevSecOps Engineer. I have been coding in one form or another for most of my adult life. I coded about 75% of the non frontend side of this app myself.

I am also a staunch opponent of AI. In general, while I think the technology could have lots of useful applications, the prevailing situation in the world today results in multitudes of slop, misinformation, and capitalization enshitification of products. The Wealthy use AI as a tool to seperate us from what makes us human and to marginalize our work. I do not support these things.

However.

The tool itself is here to stay. Pandora's box will not be closed again. Even when the AI Bubble collapses, the tool will remain. As a developer in this world, I know that it is becoming more and more an accepted and almost *necessary* part of the job - no matter how much I wish it wasnt.

I also refuse to blindly be against something - I routinely dip my fingers back into AI every few months so that I am acutely aware of its capabilities and costs and so that I do not protest against it from a position of 'feelings' and headline knee-jerk reactions.

Hence this app. 

As noted, I personally wrote about 75% of the major work of this app, exculding anything GUI related. It sat mostly unfinished for months until I decided I needed to look into Agentic Coding again at a deeper level than before. So I got Claude set up, pointed it at the app repo I already had, and went to work.  Everything added to this app after Oct 24th, 2026 is AI generated or assisted. Everything from before that is as home-grown code as you can get, complete with bugs and mistakes. 

I completely believe that people of experience in designing and building systems will make the best use of tools like these - One of my largest positions against them is how dangerous they can be - they do what you ask. If you do not know to ask, they do not do. (And... even that is being eroded)

If you dislike AI assited coding then there is absolutely zero issue from me if you choose to never use this tool. I cannot blame you. 

# Caveats while using GoSleuth

Eve Killmails are *only* deaths. They show nothing of survivors. ISK lost is only metric of a fights outcome - objectives are not visible on a killmail. And despite our requests, Logi still isnt.

This creates, in effect, the same outcome as Survivor's Bias - you only see one side of the equation. You cannot see the fights that did not result in a killmail. You cannot see the half of the fight that occured on the otherside of a stargate/wormhole. There is a lot of information you do not know.

GoSleuth also only collates data into an easily readable format. Interpreting that data is still up to you. 

# Things to do

* Pilot:
 * Fits

* Corporation/Alliances
 * Fits
 * Add to the allies list a url generation for a search on zkill that includes the main or holding and the target allies

* Systems

* Other
 * Update the main search to no longer require the dropdown for time, instead use the ESI endpoint to determine type - if more than one returned open a selection box.
  * potenitally move the search parameter preferences to a pop out button next to the search bar that can be quickly adjusted per search?
 * Add a window that can be popped out on its own that can contain running live kill trackers - Per system, Per Entity, Per Types (such as all citadels).
  * have an option to pull the kills since the tracker was last open
 * User data explorer - see all Main-Holding connections, all ignored allies of a main, and edit or add new ones.
  * have it be exportable to give to other users
 * Report data to be saved as a PDF for distribution.
 * Debug mode that outputs debugging logs to a file that can be sent to me for analysis

* Release
 * Version number in title bar
 * Version easily bumpable outside of code
 * GitActions workflows to build and release