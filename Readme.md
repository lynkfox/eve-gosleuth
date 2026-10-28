# EVE Online GoSleuth

GoSleuth is an Eve Online Intel tool. Give it a target and it will pull killmails related to that - be it Pilot, Corp, Alliance, or System - and it will display the data in meaningful ways. This is useful for looking for alts/associates of a pilot, or where they tend to hang out, among many other things.

In the grand tradition of eve tools like PyFa being coded in Python and using Py in its name, GoSleuth is coded in Go-lang.

## Yes. This app used AI in its development

*the following is left as i wrote it, with some mistakes in spelling, punctuation, capitlization so that it is clear that **I** wrote it, not the ai. This readme is my words, my thoughts, my expressed opinions. I'm human damnit, you cant get rid of me that easily you clankers!*

I am a coder by profession, a Senior DevSecOps Engineer. I have been coding in one form or another for most of my adult life. What I had coded originally probably covered about 75% of my intended features, sans the front end.

I am also a staunch opponent of AI. In general, while I think the technology could have lots of useful applications, the prevailing situation in the world today results in multitudes of slop, misinformation, and capitalization enshitification of products. The Wealthy use AI as a tool to seperate us from what makes us human and to marginalize our work. I do not support these things.

However.

The tool itself is here to stay. Pandora's box will not be closed again. Even when the AI Bubble collapses, the tool will remain. As a developer in this world, I know that it is becoming more and more an accepted and almost *necessary* part of the job - no matter how much I wish it wasn.t.

I also refuse to blindly be against something - I routinely dip my fingers back into AI every few months so that I am acutely aware of its capabilities and costs and so that I do not protest against it from a position of 'feelings' and headline knee-jerk reactions.

After a lot of talks, arguments, strained friendships, and almost lost friends, I decided that I had to dip my toe in again - so I hooked up Claude Opus 5.5 and had it take a look. 

Before GoSleuth, I wrote a tool in python I called eve-info. This was used internally by my corporation and myself and it had pretty much all the same functionality that GoSleuth has.  I wrote that tool entirely by hand and it worked. I knew I wanted to port it Go in order to take advantage of the asyncronous go-routines and in general much better speed of handling things. 

As noted, I personally wrote over 3000 lines of code (not just whitespaces and brackets lol) and that put me at roughly 75% of where I wanted the original backend to be for this iteration of eve-info, pre Gui status; Of todays non gui related code, probably 25% is either mine or based heavily on mine, and that number will undoubtedly fall as I release updates and new features. It sat mostly unfinished for months until I decided I needed to look into Agentic Coding again at a deeper level than before. So I got Claude set up, pointed it at the app repo I already had, and went to work.  Everything added to this app after Oct 24th, 2026 is AI generated or assisted. Everything from before that is as home-grown code as you can get, complete with bugs and mistakes. 

I completely believe that people of experience in designing and building systems will make the best use of tools like these - One of my largest positions against them is how dangerous they can be - they do what you ask. If you do not know to ask, they do not do. (And... even that is being eroded)

If you dislike AI assited coding then there is absolutely zero issue from me if you choose to never use this tool. I cannot blame you. 

# Caveats while using GoSleuth

Eve Killmails are *only* deaths. They show nothing of survivors. ISK lost is only metric of a fights outcome - objectives are not visible on a killmail. And despite our requests, Logi still isnt.

This creates, in effect, the same outcome as Survivor's Bias - you only see one side of the equation. You cannot see the fights that did not result in a killmail. You cannot see the half of the fight that occured on the otherside of a stargate/wormhole. There is a lot of information you do not know.

GoSleuth also only collates data into an easily readable format. Interpreting that data is still up to you. 

# Things to do

* Pre-Release

 * Fits 
  * Tags (Nano, Armor Fleet, Armor Kitchen Sink, Roller, ect) (planned currently, but not implimented)
  * Get expected resist holes, ewar resistance
 * first time user tour
  * overlay that darkens screen, highlights buttons with descriptions
  * generates sample search (fake the search?) and shows off the various tabs.

* If i feel like it pre-release?
 * Side tools: Station ID age checker
 * addition to the debugger to have an option for a live streaming of debug logs in a side window
 * Report data to be saved as a PDF for distribution.

* Release
 * GitActions workflows to build and release
 * Auto Update-er and Update changelong Splash screen

* Future plans
 * Add a window that can be popped out on its own that can contain running live kill trackers - Per system, Per Entity, Per Types (such as all citadels).
  * have an option to pull the kills since the tracker was last open
 * Ability to use killmail dumps to bypass zkill calls, SDE dumps as well
  * sde dumps is kinda already there as a dev side tool for deployment updates. Debatable if ship with app or just rely on the override system
 * Battles that the group has fought (we're already tracking, so just a display update)
  * needs warnings/reminders that its an estimation and may be wrong

