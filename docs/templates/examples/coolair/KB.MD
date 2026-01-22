# CoolAir HVAC Knowledge Base

> **AI Chat Agent Reference**
> Last Updated: January 2026

---

## Company Overview

**CoolAir HVAC** is your trusted heating and cooling specialists serving the greater metro area. We provide installation, repair, and maintenance services for residential and commercial HVAC systems.

| Attribute | Details |
|-----------|---------|
| **Company Name** | CoolAir HVAC |
| **Service Type** | Heating, Ventilation & Air Conditioning |
| **Certifications** | EPA Certified, NATE Certified Technicians |
| **Availability** | 24/7 Emergency Service |

### Our Commitment

Keeping you comfortable year-round. Fast, reliable service with upfront pricing.

---

## Contact Information

| Type | Value |
|------|-------|
| **Emergency Phone** | **(555) 987-6543** |
| **Main Office** | (555) 987-6540 |
| **Email** | service@coolairhvac.com |
| **Address** | 123 Industrial Blvd, Suite 100, Anytown, ST 12345 |

---

## Services

### Heating Repair
<!-- @service: heating_repair, emergency: true -->

Emergency and scheduled repair for all heating systems including furnaces, heat pumps, and boilers.

**We Repair:**
- Gas and electric furnaces
- Heat pumps
- Boilers
- Radiant heating systems
- Ductless mini-splits

**Response Time:** Same day for emergencies, especially during cold weather

---

### AC Repair
<!-- @service: ac_repair, emergency: true -->

Air conditioning repair for central AC, heat pumps, and ductless systems.

**We Repair:**
- Central air conditioning
- Heat pumps (cooling mode)
- Ductless mini-split systems
- Commercial HVAC units

**Response Time:** Same day during heat waves and peak summer

---

### Furnace Installation
<!-- @service: furnace_install, emergency: false -->

Professional installation of new furnaces with proper sizing and efficient setup.

**Includes:**
- Load calculation for proper sizing
- Removal of old equipment
- Professional installation
- System testing and calibration
- Warranty registration

---

### AC Installation
<!-- @service: ac_install, emergency: false -->

New air conditioning installation including central AC and ductless systems.

---

### Heat Pump Installation
<!-- @service: heat_pump_install, emergency: false -->

Energy-efficient heat pump installation for year-round heating and cooling.

---

### Maintenance Plans
<!-- @service: maintenance, emergency: false -->

Annual and semi-annual maintenance plans to keep your system running efficiently and catch problems early.

**Benefits:**
- Priority scheduling
- Discounts on repairs
- Extended equipment life
- Lower energy bills
- Fewer breakdowns

---

### Duct Cleaning
<!-- @service: duct_cleaning, emergency: false -->

Professional air duct cleaning to improve air quality and system efficiency.

---

### Thermostat Installation
<!-- @service: thermostat_install, emergency: false -->

Smart and programmable thermostat installation and setup.

---

## Services We Don't Provide

### Plumbing
<!-- @exclusion: plumbing -->

We specialize in HVAC systems, not plumbing. This includes water heaters (unless it's a hydronic heating system).

**Recommendation:** Contact a licensed plumber.

---

### Electrical Panel Work
<!-- @exclusion: electrical, exception: "unless it's directly related to HVAC equipment" -->

We don't do general electrical work like panel upgrades or rewiring.

**Recommendation:** Contact a licensed electrician.

---

### Appliance Repair
<!-- @exclusion: appliances -->

We don't repair refrigerators, washers, dryers, or other appliances.

**Recommendation:** Contact an appliance repair service.

---

## Service Areas

### Areas We Serve
<!-- @coverage: include -->

**Metro Area:**
Downtown, Northside, Southside, Eastwood, Westville, Riverside, Lakewood, Oak Park, Maple Heights, Cedar Grove, Pine Hills, Hillcrest, Valley View, Sunrise, Sunset Hills

**Surrounding Towns:**
Springfield, Centerville, Fairview, Georgetown, Hamilton, Jefferson, Madison, Monroe, Washington, Lincoln

---

### Areas Outside Our Coverage
<!-- @coverage: exclude -->

We do not currently service:
- Rural County (too far)
- Mountain Region

---

## Frequently Asked Questions

### How quickly can you come out?
<!-- @faq: response_time -->

For emergencies like no heat in winter or no AC during a heat wave, we offer **same-day service**. For routine repairs and maintenance, we can usually schedule within 1-2 days.

---

### How much does a repair cost?
<!-- @faq: pricing -->

We provide **upfront pricing** before any work begins. A diagnostic fee applies to determine the problem (waived if you proceed with repair). We'll give you the exact cost before starting work - no surprises.

---

### Do you offer financing?
<!-- @faq: financing -->

Yes! We offer financing options for new equipment installations. Ask about our 0% interest plans for qualified buyers.

---

### How often should I service my HVAC?
<!-- @faq: maintenance_frequency -->

We recommend **twice yearly** - once in spring before cooling season, and once in fall before heating season. Our maintenance plans make this easy and save you money.

---

### Why is my furnace blowing cold air?
<!-- @faq: cold_air -->

This could be several things - thermostat settings, pilot light issues, ignition problems, or airflow issues. Best to have a technician diagnose it. If you have no heat at all, call our emergency line.

---

### What's that burning smell when I turn on the heat?
<!-- @faq: burning_smell -->

A slight dusty/burning smell when you first turn on heat for the season is normal - it's dust burning off. If it persists or smells like gas or electrical burning, turn off the system and call us immediately.

---

## Emergency Safety Procedures

### No Heat Emergency (Cold Weather)
<!-- @safety: no_heat_emergency, triggers: emergency_no_heat, emergency_heating -->

**If you have no heat during freezing temperatures:**

1. **Check your thermostat** - Make sure it's set to "Heat" and the temperature is set higher than current room temp
2. **Check the circuit breaker** - Reset if tripped
3. **Check your filter** - A clogged filter can cause shutdown
4. **If gas furnace:** Check if pilot light is on (older models)
5. **Call our emergency line:** **(555) 987-6543**

**While waiting:**
- Use space heaters safely (keep away from flammables)
- Open faucets to a trickle to prevent pipe freezing
- If you have elderly family members or infants, consider going somewhere warm

---

### Gas Smell Emergency
<!-- @safety: gas_emergency, triggers: emergency_gas -->

**If you smell gas:**

1. **Do NOT flip any light switches or create sparks**
2. **Do NOT use your phone inside**
3. **Leave the building immediately**
4. **Call your gas company from outside**
5. **Do NOT re-enter** until cleared by gas company

This is a safety emergency - gas company first, then call us for HVAC inspection after it's safe.

---

### No AC Emergency (Extreme Heat)
<!-- @safety: no_ac_emergency, triggers: emergency_no_ac, emergency_ac -->

**If you have no AC during a heat wave:**

1. **Check thermostat** - Make sure it's set to "Cool" and set lower than room temp
2. **Check circuit breaker** - Both indoor and outdoor units
3. **Check air filter** - Replace if dirty
4. **Make sure outdoor unit** isn't blocked by debris
5. **Call us:** **(555) 987-6543** - We prioritize extreme heat calls

**While waiting:**
- Stay hydrated
- Use fans
- Close blinds on sunny windows
- Go to a cool location if unsafe at home (elderly, health conditions)

---

### Carbon Monoxide Warning
<!-- @safety: co_emergency, triggers: emergency_co -->

**If your CO detector goes off:**

1. **Get everyone out of the house immediately**
2. **Call 911**
3. **Do NOT go back inside**
4. **Call us after fire department clears you** - We'll inspect your heating system

**Symptoms of CO poisoning:** Headache, dizziness, nausea, confusion. Get fresh air immediately.

---

## Additional Information

### Brands We Service

We service all major brands including Carrier, Trane, Lennox, Rheem, Goodman, Bryant, American Standard, York, and more.

### Our Guarantee

- Upfront pricing - no surprises
- 100% satisfaction guaranteed
- Licensed and insured
- NATE certified technicians
- 1-year warranty on repairs

---

*CoolAir HVAC - Keeping You Comfortable Year-Round*
*EPA & NATE Certified | 24/7 Emergency Service*
