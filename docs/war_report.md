# War Report

Are you engaged in a big war with another corp, alliance, or even a coalition? Would you like to track your isk lost and killed, so you can rally the troops and dishearten your enemies? Look no further than the War Report!

## Tracking a war

The **War** button sets up two sides and a start date, and GoSleuth follows the fighting between them. Each side can be a coalition, an affiliate group, or one corp or alliance. Wars are saved, and open again with no searching at all.

### How Killmails are sorted for the war

- **What counts**
  - *The search filters through killmails from both sides back to the start date - so the earlier you start tracking the war, the easier the searches will go.*
  - *Only kills where pilots form both sides were on them in a collected 'battle' count - so if one of your sides goes of on a null roam and kills a bunch of goons, it wont count for your wormhole war*
  - *Even still, some of those others may show up - they can be ingored by right clicking them on the Overview and Participants pages*
- **Station trash and asset denial**
  - *The tool attempts to label kms as Asset Denial or Station trash. Given that there is no real way to tell other than human intuition, these are marked as either probable or uncertain.*
  - ***Station Trash***
    - *Kills in rapid succession from the same pilot with no capsule km, in a system where a station recently died are labled as Station trash and probable*
    - *Anchorables, such as Mobile Warp Dsruptors, are also considered trash*
    - *Note: Mobile Tractor Units are exempt from that*
    - *Single pilot kills, within 4 hours of a station kill, espcially with no pod, are labled as uncertain*
  - ***Asset Denial***
    - *Asset Denial has to happen before a structure is destroyed in the same system*
    - *A Capital with nothing other than rigs equiped, where the same sides station is part of the killmail. If the station is top damage or there are only blue attackers, its listed as probable*
    - *A Capital with mix of blue and red attckers, but without the station being top damage is considered uncertain.*
    - *A Capital with no blues on it is never Asset denial*
    - *Subcapitals are only considered Asset denial if a blue station is the only attacker*
    - *Capsules are never counted as asset denial*
- **Station ref and Ref attempt**
  - *A citadel being ref'd does not show up by itself on killmails*
  - *The search attempts to show them if it finds a battle where Standup Fighters are used or a station not destroyed in the battle participates in the fight.*
    - *This very well will miss ref attempts! It also does not say if it is successful or not*

### The various report pages

- **Overview**
  - *Isk Lost, Ships lost, killmails, structures lost, and more information at a glance*
- **Participates**
  - **Forthcoming**
  - *A list of corps that can be expanded to show individual pilots per side.*
- **Battles**
  - *A listing of fights - that may stretch multiple systems. Each line reports the group that had the most pilots for their respective side, and Stations killed.*
  - *Skirmishes are less than 10 pilots a side*
  - *The listing can be sorted by column and filtered by system class.*
  - *Links to various BR sites are available, but since there is no automated way to generate multi system reports, it only does so for the first system*
- **Systems**
  - *Similar to the Battles, but showing everything for a given system*
- **Timeline**
  - *What each side lost by day or by week, as that period's figures or running totals*


Killmails can't show objectives or survivors, so GoSleuth never calls a battle won.