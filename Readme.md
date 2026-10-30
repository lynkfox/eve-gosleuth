# EVE Online GoSleuth

GoSleuth is an Eve Online Intel tool. Give it a target and it will pull killmails related to that - be it Pilot, Corp, Alliance, or System - and it will display the data in meaningful ways. This is useful for looking for alts/associates of a pilot, or where they tend to hang out, among many other things.

In the grand tradition of eve tools like **Py**Fa being coded in **Py**thon and using **Py** in its name, **Go**Sleuth is coded in **Go**-lang.

[Click Here for a Full Feature List](docs/features.md)

## Installing

* Go to the [release page](https://github.com/lynkfox/eve-gosleuth/releases) and download the zip (you dont need any of the other files)
* Extract it wherever you wish.
* Open it
 * If you get a warning window hit "More Info" and then "Run Anyway"
  * *the app is safe, its open source, stick it in any virus scanner you want, point an Ai at the repo to analyze it, whatever you feel you need to do.*

### Updating

* When a new version releases, simply replace the exe with the new one.
* Your intel assocations, preferences, and cached searches will be retained
* If youve made any changes to the static_data overrides, youll be fine uniless its a Major change (x.0.0 where the x goes up a number). 
* If that happens, then we'll be sure to offer guidance.

## Always Free, Always Open

GoSleuth will always be free, open source, free of ads, subscriptions, paid feature tiers, ect. This app runs entirely in your local environment and connects to no servers beyond the standard apis that are listed in the Data Sources. This app strives to maintain your privacy - if you feel that it is not for some reason, please submit a bug report on the issues page of this repo. 

If you feel this app has been worth your time, feel free to - with no obligation or expectation - donate some Isk or Plex to Luxxianna Seraph in game.


# Caveats while using GoSleuth

Eve Killmails are *only* deaths. They show nothing of survivors. ISK lost is only metric of a fights outcome - objectives are not visible on a killmail. And despite our requests, Logi still isnt.

This creates, in effect, the same outcome as Survivor's Bias - you only see one side of the equation. You cannot see the fights that did not result in a killmail. You cannot see the half of the fight that occured on the otherside of a stargate/wormhole. There is a lot of information you do not know.

GoSleuth also only collates data into an easily readable format. Interpreting that data is still up to you. 


# Yes. This app used AI in its development

*the following is left as i wrote it, with some mistakes in spelling, punctuation, capitlization so that it is clear that **I** wrote it, not the ai. This readme is my words, my thoughts, my expressed opinions. I'm human damnit, you cant get rid of me that easily you clankers!*

I am a coder by profession, a Senior DevSecOps Engineer. I have been coding in one form or another for most of my adult life. What I had coded originally probably covered about 75% of my intended features, sans the front end.

I am also a staunch opponent of AI. In general, while I think the technology could have lots of useful applications, the prevailing situation in the world today results in multitudes of slop, misinformation, and capitalization enshitification of products. The Wealthy use AI as a tool to seperate us from what makes us human and to marginalize our work. I do not support these things.

However.

The tool itself is here to stay. Pandora's box will not be closed again. Even when the AI Bubble collapses, the tool will remain. As a developer in this world, I know that it is becoming more and more an accepted and almost *necessary* part of the job - no matter how much I wish it wasn.t.

I also refuse to blindly be against something - I routinely dip my fingers back into AI every few months so that I am acutely aware of its capabilities and costs and so that I do not protest against it from a position of 'feelings' and headline knee-jerk reactions.

After a lot of talks, arguments, strained friendships, and almost lost friends, I decided that I had to dip my toe in again - so I hooked up Claude Opus 5.5 and had it take a look. 

Before GoSleuth, I wrote a tool in python I called eve-info. This was used internally by my corporation and myself and it had pretty much all the same functionality that GoSleuth has.  I wrote that tool entirely by hand and it worked. I knew I wanted to port it Go in order to take advantage of the asyncronous go-routines and in general much better speed of handling things. 

As noted, I personally wrote over 3000 lines of code+tests (not just whitespaces and brackets lol) and that put me at roughly 75% of where I wanted the original backend to be for this iteration of eve-info, pre Gui status; Of todays non gui related code, probably 25% is either mine or based heavily on mine, and that number will undoubtedly fall as I release updates and new features. It sat mostly unfinished for months until I decided I needed to look into Agentic Coding again at a deeper level than before. So I got Claude set up, pointed it at the app repo I already had, and went to work.  Everything added to this app after Oct 24th, 2026 is AI generated or assisted. Everything from before that is as home-grown code as you can get, complete with bugs and mistakes. 

I completely believe that people of experience in designing and building systems will make the best use of tools like these - One of my largest positions against them is how dangerous they can be - they do what you ask. If you do not know to ask, they do not do. (And... even that is being eroded)

If you dislike AI assited coding then there is absolutely zero issue from me if you choose to never use this tool. I cannot blame you. 



# Things to do

Roadmap of planned features.

* Next
 * Auto Update-er and Update changelong Splash screen
 * first time user tour
  * overlay that darkens screen, highlights buttons with descriptions
  * generates sample search (fake the search?) and shows off the various tabs.
 * Battles that the group has fought (we're already tracking, so just a display update)
  * needs warnings/reminders that its an estimation and may be wrong
  * and inclusion in PDF report with links to eve tools, war beacon, and zkill battle pages

* A bit further out
 * On the battles tab, ability to tag a specific kill as a Fleet Doctrine from the battle pages for future refrence (saved in My Intel)
 * Jspace Home Systems / Nullsec Staging added to Affilliates system

* Future plans
 * wails v3 upgrade for multiple windows (side tools)
  * Side Tools
    * Killtrackers, based on any of the searchable targets in seperate windows
     * ability to pull killtrackers data since last open
     * using their rss feed
    * ZKill search reports - Set up several search parameters that can be quickly run as a report
     * Ie: every structure kill in jspace since the report was last run
     * or: every structure kill involving a, b, and c, in jspace in the last day
     * ect
    * Station ID checker, for date station was anchored (ala trifftools)
    * Live logging debug logs for more advanced users
 * Ability to use killmail dumps to bypass zkill calls, SDE dumps as well
  * sde dumps is kinda already there as a dev side tool for deployment updates. Debatable if ship with app or just rely on the override system
 * API/MCP for command line and ai integration calls
 * Translation system (Not translated by me but translation dictionaries)
