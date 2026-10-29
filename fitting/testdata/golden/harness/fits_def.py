# Fit definitions: (file name, EFT text, tactical-destroyer?)  -- no drones, no boosters, no mutated modules.
# EFT order: lows, mids, highs, rigs, subsystems, (blank line) implants.
FITS = []
def fit(name, eft, td=False):
    FITS.append((name, eft.strip("\n") + "\n", td))

fit("tengu_shield", """
[Tengu, tengu_shield]
Damage Control II
Ballistic Control System II
Ballistic Control System II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
10MN Afterburner II

Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II
Medium Core Defense Field Extender II

Tengu Defensive - Supplemental Screening
Tengu Core - Electronic Efficiency Gate
Tengu Offensive - Accelerated Ejection Bay
Tengu Propulsion - Fuel Catalyst
""")

fit("legion_armor", """
[Legion, legion_armor]
Damage Control II
Medium Ancillary Armor Repairer, Nanite Repair Paste
800mm Steel Plates II
Multispectrum Energized Membrane II
Thermal Armor Hardener II
Kinetic Armor Hardener II

Medium Ancillary Shield Booster, Cap Booster 100
Sensor Booster II, ECCM Script

Dual Heavy Pulse Laser II
Dual Heavy Pulse Laser II
Dual Heavy Pulse Laser II
Dual Heavy Pulse Laser II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
Medium Auxiliary Nano Pump II

Legion Defensive - Augmented Plating
Legion Core - Augmented Antimatter Reactor
Legion Offensive - Liquid Crystal Magnifiers
Legion Propulsion - Intercalated Nanofibers
""")

fit("loki_armor", """
[Loki, loki_armor]
Damage Control II
Medium Armor Repairer II
EM Armor Hardener II
Explosive Armor Hardener II
Multispectrum Energized Membrane II

Stasis Webifier II
Warp Scrambler II
10MN Afterburner II

200mm AutoCannon II
200mm AutoCannon II
200mm AutoCannon II
200mm AutoCannon II
200mm AutoCannon II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
Medium Auxiliary Nano Pump II

Loki Defensive - Adaptive Defense Node
Loki Core - Dissolution Sequencer
Loki Offensive - Projectile Scoping Array
Loki Propulsion - Intercalated Nanofibers
""")

fit("proteus_armor", """
[Proteus, proteus_armor]
Damage Control II
Medium Armor Repairer II
Thermal Armor Hardener II
Kinetic Armor Hardener II
Multispectrum Energized Membrane II

Sensor Booster II
Warp Scrambler II
10MN Afterburner II

Neutron Blaster Cannon II
Neutron Blaster Cannon II
Neutron Blaster Cannon II
Neutron Blaster Cannon II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
Medium Auxiliary Nano Pump II

Proteus Defensive - Nanobot Injector
Proteus Core - Augmented Fusion Reactor
Proteus Offensive - Hybrid Encoding Platform
Proteus Propulsion - Localized Injectors
""")

# Tactical destroyers, one file per mode (the harness writes <name>_defense / _sharpshooter / _propulsion)
fit("confessor", """
[Confessor, confessor]
Damage Control II
Small Armor Repairer II
EM Armor Hardener II
Multispectrum Energized Membrane II

Small Shield Extender II
Warp Scrambler II

Heat Sink II

Small Trimark Armor Pump II
""", td=True)

fit("jackdaw", """
[Jackdaw, jackdaw]
Damage Control II
Ballistic Control System II
Ballistic Control System II

Small Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Warp Scrambler II

Rocket Launcher II
Rocket Launcher II
Rocket Launcher II
Rocket Launcher II
Rocket Launcher II

Small Core Defense Field Extender II
Small Core Defense Field Extender II
""", td=True)

fit("damnation", """
[Damnation, damnation]
Damage Control II
Large Armor Repairer II
1600mm Steel Plates II
EM Armor Hardener II
Thermal Armor Hardener II
Multispectrum Energized Membrane II

Sensor Booster II
Large Shield Extender II
10MN Afterburner II

Armor Command Burst II, Rapid Repair Charge
Armor Command Burst II, Armor Energizing Charge
Information Command Burst II
Heavy Assault Missile Launcher II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("vulture", """
[Vulture, vulture]
Damage Control II
Signal Amplifier II
Ballistic Control System II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Large Shield Booster II
Sensor Booster II, ECCM Script

Shield Command Burst II, Shield Harmonizing Charge
Information Command Burst II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II
""")

fit("golem_bastion", """
[Golem, golem_bastion]
Damage Control II
Ballistic Control System II
Ballistic Control System II
Ballistic Control System II

X-Large Shield Booster II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Shield Boost Amplifier II
Sensor Booster II

Bastion Module I
Torpedo Launcher II
Torpedo Launcher II
Torpedo Launcher II

Large Core Defense Field Extender II
Large Core Defense Field Extender II

""")

fit("paladin_bastion", """
[Paladin, paladin_bastion]
Damage Control II
Large Armor Repairer II
Large Armor Repairer II
EM Armor Hardener II
Thermal Armor Hardener II
Multispectrum Energized Membrane II

Sensor Booster II
10MN Afterburner II

Bastion Module I
Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II

Large Trimark Armor Pump II
Large Trimark Armor Pump II

""")

fit("revelation_siege", """
[Revelation, revelation_siege]
Capital Armor Repairer II
Damage Control II
EM Armor Hardener II
Thermal Armor Hardener II
Multispectrum Energized Membrane II
Capital Nanobot Accelerator II

Capital Shield Extender II
Sensor Booster II

Siege Module II

Capital Trimark Armor Pump II
Capital Trimark Armor Pump II

""")

fit("minokawa_triage", """
[Minokawa, minokawa_triage]
Damage Control II
Capital Armor Repairer II
EM Armor Hardener II
Thermal Armor Hardener II

Capital Remote Capacitor Transmitter II
Sensor Booster II
Capital Shield Extender II

Triage Module II
Capital Remote Armor Repairer II
Capital Remote Armor Repairer II
Capital Remote Armor Repairer II
Capital Remote Armor Repairer II

Capital Auxiliary Nano Pump II
Capital Trimark Armor Pump II
Capital Trimark Armor Pump II
""")

fit("guardian_logi", """
[Guardian, guardian_logi]
Damage Control II
Medium Armor Repairer II
Thermal Armor Hardener II
Multispectrum Energized Membrane II

Sensor Booster II, ECCM Script
Medium Shield Extender II

Medium Remote Armor Repairer II
Medium Remote Armor Repairer II
Medium Remote Armor Repairer II
Medium Remote Capacitor Transmitter II
Medium Remote Capacitor Transmitter II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("basilisk_logi", """
[Basilisk, basilisk_logi]
Damage Control II
Signal Amplifier II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Sensor Booster II, ECCM Script
10MN Afterburner II

Medium Remote Shield Booster II
Medium Remote Shield Booster II
Medium Remote Shield Booster II
Medium Remote Capacitor Transmitter II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II
""")

fit("scimitar_logi", """
[Scimitar, scimitar_logi]
Damage Control II
Signal Amplifier II
Signal Amplifier II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Medium Shield Booster II
Stasis Webifier II

Medium Remote Shield Booster II
Medium Remote Shield Booster II
Medium Remote Shield Booster II
Medium Remote Capacitor Transmitter II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II
""")

fit("deacon_logi", """
[Deacon, deacon_logi]
Damage Control II
Small Armor Repairer II
Multispectrum Energized Membrane II

Small Shield Extender II
Sensor Booster II

Small Remote Armor Repairer II
Small Remote Armor Repairer II
Small Remote Hull Repairer II

Small Trimark Armor Pump II
Small Auxiliary Nano Pump II
""")

fit("dc_hardeners_abaddon", """
[Abaddon, dc_hardeners_abaddon]
Damage Control II
1600mm Steel Plates II
EM Armor Hardener II
Thermal Armor Hardener II
Kinetic Armor Hardener II
Multispectrum Energized Membrane II

10MN Afterburner II
Sensor Booster II

Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II

Large Trimark Armor Pump II
Large Trimark Armor Pump II
Large Auxiliary Nano Pump II
""")

fit("assault_dc_zealot", """
[Zealot, assault_dc_zealot]
Assault Damage Control II
EM Armor Hardener II
Thermal Armor Hardener II
Kinetic Armor Hardener II
Multispectrum Energized Membrane II
Medium Armor Repairer II

10MN Afterburner II

Heavy Pulse Laser II
Heavy Pulse Laser II
Heavy Pulse Laser II
Heavy Pulse Laser II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("rah_dc_abaddon", """
[Abaddon, rah_dc_abaddon]
Damage Control II
Reactive Armor Hardener
1600mm Steel Plates II
Multispectrum Energized Membrane II

Sensor Booster II
10MN Afterburner II

Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II

Large Trimark Armor Pump II
Large Trimark Armor Pump II
Large Auxiliary Nano Pump II
""")

fit("rah_hardener_dc_apocalypse", """
[Apocalypse, rah_hardener_dc_apocalypse]
Damage Control II
Reactive Armor Hardener
Kinetic Armor Hardener II
1600mm Steel Plates II
Multispectrum Energized Membrane II

Sensor Booster II
10MN Afterburner II

Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II
Mega Pulse Laser II

Large Trimark Armor Pump II
Large Trimark Armor Pump II
Large Auxiliary Nano Pump II
""")

fit("rah_assault_dc_deimos", """
[Deimos, rah_assault_dc_deimos]
Assault Damage Control II
Reactive Armor Hardener
Medium Armor Repairer II
Multispectrum Energized Membrane II

10MN Afterburner II

Heavy Ion Blaster II
Heavy Ion Blaster II
Heavy Ion Blaster II
Heavy Ion Blaster II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("aar_paste_drake", """
[Drake, aar_paste_drake]
Damage Control II
Medium Ancillary Armor Repairer, Nanite Repair Paste
Thermal Armor Hardener II
Multispectrum Energized Membrane II

10MN Afterburner II
Large Shield Extender II

Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("aar_nopaste_drake", """
[Drake, aar_nopaste_drake]
Damage Control II
Medium Ancillary Armor Repairer
Thermal Armor Hardener II
Multispectrum Energized Membrane II

10MN Afterburner II
Large Shield Extender II

Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II
""")

fit("asb_cap_booster_drake", """
[Drake, asb_cap_booster_drake]
Damage Control II
Ballistic Control System II
Ballistic Control System II

Large Ancillary Shield Booster, Cap Booster 200
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Shield Boost Amplifier II

Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II
""")

fit("passive_shield_cyclone", """
[Cyclone, passive_shield_cyclone]
Damage Control II
Power Diagnostic System II

Large Shield Extender II
Shield Power Relay II
Shield Power Relay II
Shield Flux Coil II
Multispectrum Shield Hardener II

650mm Artillery Cannon II
650mm Artillery Cannon II

Medium Core Defense Operational Solidifier II
Medium Core Defense Operational Solidifier II
Medium Core Defense Field Extender II
""")

fit("hull_tank_catalyst", """
[Catalyst, hull_tank_catalyst]
Damage Control II
Reinforced Bulkheads II
Small Hull Repairer II

Small Shield Extender II

Light Neutron Blaster II
Light Neutron Blaster II
Light Neutron Blaster II
Light Neutron Blaster II

Small Transverse Bulkhead II
Small Transverse Bulkhead II
""")

fit("plain_rifter", """
[Rifter, plain_rifter]
200mm AutoCannon II
200mm AutoCannon II
""")

fit("armor_implants_oneiros", """
[Oneiros, armor_implants_oneiros]
Damage Control II
Medium Armor Repairer II
EM Armor Hardener II
Thermal Armor Hardener II
Multispectrum Energized Membrane II

Medium Remote Capacitor Transmitter II
Sensor Booster II

Medium Remote Armor Repairer II
Medium Remote Armor Repairer II
Medium Remote Armor Repairer II

Medium Trimark Armor Pump II
Medium Trimark Armor Pump II

High-grade Asklepian Alpha
High-grade Asklepian Beta
High-grade Asklepian Gamma
High-grade Asklepian Delta
High-grade Asklepian Epsilon
Inherent Implants 'Noble' Hull Upgrades HG-1005
Inherent Implants 'Noble' Mechanic MC-805
""")

fit("shield_implants_drake", """
[Drake, shield_implants_drake]
Damage Control II
Ballistic Control System II
Ballistic Control System II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II
Shield Boost Amplifier II
Medium Shield Booster II

Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II
Heavy Assault Missile Launcher II

Medium Core Defense Field Extender II
Medium Core Defense Field Extender II

High-grade Crystal Alpha
High-grade Crystal Beta
High-grade Crystal Gamma
High-grade Crystal Delta
High-grade Crystal Epsilon
High-grade Crystal Omega
""")

fit("sensor_eccm_raven", """
[Raven, sensor_eccm_raven]
Damage Control II
Signal Amplifier II
Signal Amplifier II
Ballistic Control System II

Sensor Booster II, ECCM Script
Sensor Booster II, ECCM Script
Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II

Cruise Missile Launcher II
Cruise Missile Launcher II
Cruise Missile Launcher II
Cruise Missile Launcher II

Large Core Defense Field Extender II
Large Core Defense Field Extender II
""")

fit("triglavian_leshak", """
[Leshak, triglavian_leshak]
Damage Control II
Reinforced Bulkheads II
Medium Armor Repairer II

Large Shield Extender II
Multispectrum Shield Hardener II
Thermal Shield Hardener II

Supratidal Entropic Disintegrator II

Large Trimark Armor Pump II
Large Trimark Armor Pump II
""")

fit("herald_ewar", """
[Herald, herald_ewar]
Damage Control II
Small Armor Repairer II

Small Shield Extender II
Warp Scrambler II
""")

fit("monitor_ewar", """
[Monitor, monitor_ewar]
Medium Shield Extender II
Sensor Booster II
Multispectrum Shield Hardener II

Medium Remote Armor Repairer II
""")

fit("nyx_ewar", """
[Nyx, nyx_ewar]
Damage Control II
Capital Armor Repairer II
EM Armor Hardener II
Thermal Armor Hardener II
Multispectrum Energized Membrane II

Capital Shield Extender II
Sensor Booster II


Capital Trimark Armor Pump II
Capital Trimark Armor Pump II
""")
