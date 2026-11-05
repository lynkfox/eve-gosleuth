# War Report

Are you engaged in a big war with another corp, alliance, or even a coalition? Would you like to track your ISK lost and killed, so you can rally the troops and dishearten your enemies? Look no further than the War Report!

*Killmails can't show objectives or survivors, so GoSleuth never calls a battle won. That's for you to decide.*

## Tracking a war

The **War** button sets up two sides and a start date, and GoSleuth follows the fighting between them. Each side can be a coalition, an affiliate group, or one corp or alliance. Wars are saved, and open again with no searching at all.

- **From battle report**
  - *In the War window or on a war's report, paste a saved EVE Tools battle report link (br.evetools.org/br/...) to read both sides from it. Automatic reports (br.evetools.org/related/...) have no saved sides, so they can't be read*
  - *Each side goes in a new coalition, or is added to one you already have. Saving only ever adds: unticking a group never takes it out of a coalition*
  - *An alliance with 3 pilots or fewer on the report, or a corp with 5 or fewer, starts unticked. NPC corporations are left out*

### How Killmails are sorted for the war

- **What counts**
  - *The search filters through killmails from both sides back to the start date - the earlier in a war you start generating reports, the easier it will be for the search to keep up without being overwhelmed or getting rate limited*
  - *A 'battle' only counts if one side killed the other on at least one of its killmails - so if one of your sides goes off on a null roam and kills a bunch of goons, it won't count for your wormhole war*
  - *Even still, some of those others may show up - they can be ignored by right-clicking them on the Overview and Participants pages*
- **Station Trash and Asset Denial**
  - *The tool attempts to label kms as Asset Denial or Station Trash. Given that there is no real way to tell other than human intuition, these are marked as either probable or uncertain*
  - ***Station Trash***
    - *Kills in rapid succession from the same pilot with no capsule km, in a system where a station recently died, are labeled as Station Trash and probable*
    - *Anchorables, such as Mobile Warp Disruptors, are also considered Station Trash*
    - *Note: Mobile Tractor Units are exempt from that*
    - *Other blue losses in the same system, up to 4 hours after the battle the station died in, are labeled as uncertain - unless it looks like an actual battle (ie an escape attempt or revenge fleet)*
    - *A pilot whose capsule died within 5 minutes of their ship was in it, so those losses are never Station Trash*
  - ***Asset Denial***
    - *Asset Denial has to happen before a structure is destroyed in the same system - this can include structures surviving!*
    - *A Capital with only rigs is probable only if there is a km for a station within a few days. Otherwise it's listed as uncertain*
    - *A Capital where a blue station is top damage, no matter the other participants or its fitting, is probable*
    - *A Capital with a mix of blue and red attackers, but without the station being top damage, is considered uncertain*
    - *A Fitted Capital with no blues on it is never Asset Denial*
    - *Subcapitals are only considered Asset Denial if a blue station is the only attacker*
    - *Capsules are never counted as Asset Denial*
- **Station ref and Ref attempt**
  - *A citadel being ref'd does not show up by itself on killmails*
  - *The search attempts to show them if it finds a battle where Standup Fighters are used or a station not destroyed in the battle participates in the fight*
    - *This very well may miss ref attempts! It also does not say if it is successful or not*

### The various report pages

- **Overview**
  - *ISK lost, ships lost, killmails, structures lost, and more information at a glance*
- **Participants**
  - **Forthcoming**
  - *A list of corps that can be expanded to show individual pilots per side*
- **Battles**
  - *A listing of fights that may stretch multiple systems. Each line reports the group that had the most pilots for their respective side, and stations killed*
  - *Skirmishes are fewer than 10 pilots in the whole fight*
  - *The listing can be sorted by column and filtered by system class*
  - *Links to various BR sites are available, but since there is no automated way to generate multi-system reports, it only does so for the system with the most killmails*
- **Systems**
  - *Similar to the Battles, but showing everything for a given system*
- **Timeline**
  - *What each side lost by day or by week, as that period's figures or running totals*


