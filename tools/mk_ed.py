# Genera data/ed_commands.json: elenco dei comandi di Elite Dangerous (nome inglese, traduzione italiana, gruppo, tipo).
# Nomi inglesi brevi dal progetto EDRefCard (MIT) dove presenti; il resto e la traduzione sono nostri.
import json, re, sys
import xml.etree.ElementTree as ET
ref=json.load(open(sys.argv[1])); root=ET.parse(sys.argv[2]).getroot()
EXTRA={'BlockMouseDecay':'Hold Mouse Position','TriggerFieldNeutraliser':'Field Neutraliser','TriggerColonisationModule':'Colonisation Module',
'OpenCodexGoToDiscovery_Buggy':'Codex','PlayerHUDModeToggle_Buggy':'HUD Mode','VanityCameraTen':'Cam - Vehicle Turret','MoveFreeCamUpAxis':'Cam Up (axis)','MoveFreeCamDownAxis':'Cam Down (axis)',
'ExplorationSAANextGenus':'Next Genus (DSS)','ExplorationSAAPreviousGenus':'Prev Genus (DSS)','HumanoidWalkButton':'Walk','HumanoidEmoteWheelButton':'Emote Wheel','HumanoidUtilityWheelCycleMode':'Wheel Mode',
'HumanoidClearAuthorityLevel':'Clear Authority Level','HumanoidHealthPack':'Medkit','HumanoidBattery':'Energy Cell','HumanoidSelectFragGrenade':'Frag Grenade','HumanoidSelectEMPGrenade':'EMP Grenade','HumanoidSelectShieldGrenade':'Shield Grenade','HumanoidPing':'Ping',
'PlaceSettlement':'Place Settlement','ChangeConstructionOption':'Change Construction Option','RotateSettlement':'Rotate Settlement','RotateSettlementLeft':'Rotate Settlement Left','RotateSettlementRight':'Rotate Settlement Right','ExitSettlementPlacementCamera':'Exit Placement Cam',
'MovePlacementCamY':'Placement Cam Y','MovePlacementCamForward':'Placement Cam Forwards','MovePlacementCamBackwards':'Placement Cam Backwards','MovePlacementCamX':'Placement Cam X','MovePlacementCamRight':'Placement Cam Right','MovePlacementCamLeft':'Placement Cam Left','MovePlacementCamZ':'Placement Cam Z','MovePlacementCamUp':'Placement Cam Up','MovePlacementCamDown':'Placement Cam Down','MovePlacementCamUpAxis':'Placement Cam Up (axis)','MovePlacementCamDownAxis':'Placement Cam Down (axis)',
'PitchPlacementCamera':'Placement Cam Pitch','PitchPlacementCameraUp':'Placement Cam Pitch Up','PitchPlacementCameraDown':'Placement Cam Pitch Down','YawPlacementCamera':'Placement Cam Yaw','YawPlacementCameraRight':'Placement Cam Yaw Right','YawPlacementCameraLeft':'Placement Cam Yaw Left','PlacementCamSpeedInc':'Inc Placement Cam Speed','PlacementCamSpeedDec':'Dec Placement Cam Speed'}
for i in range(1,9): EXTRA['HumanoidEmoteSlot%d'%i]='Emote %d'%i
FIX={'YawAxis_Landing':'Yaw (Lndg)','AheadThrust':'Forward Thrust (axis)','AheadThrust_Landing':'Forward Thrust (axis) (Lndg)','ForwardKey':'Throttle Up','BackwardKey':'Throttle Down','HumanoidZoomButton':'Aim / Zoom','ToggleReverseThrottleInput':'Reverse Throttle','BuggyToggleReverseThrottleInput':'Reverse Throttle'}
# traduzioni complete (nomi che non si traducono bene parola per parola)
FULL={'Reset Mouse':'Ricentra mouse','Hold Mouse Position':'Blocca posizione mouse','Yaw To Roll':'Imbardata in rollio','Alternate Controls':'Comandi alternativi','Throttle':'Manetta','Reverse Throttle':'Inverti manetta','Throttle Up':'Aumenta manetta','Throttle Down':'Diminuisci manetta',
'All Stop':'Ferma (0%)','Flight Assist':'Assistenza al volo','Boost':'Boost','Hyperspace/Supercruise':'Iperspazio / Supercruise','Supercruise':'Supercruise','Hyperspace':'Iperspazio','Rotational Correction':'Correzione rotazionale','Orbit Lines':'Linee orbitali',
'Target Ahead':'Bersaglio davanti','Next Contact':'Contatto successivo','Prev Contact':'Contatto precedente','Highest Threat':'Minaccia maggiore','Next Hostile':'Ostile successivo','Prev Hostile':'Ostile precedente','Wingman 1':'Gregario 1','Wingman 2':'Gregario 2','Wingman 3':'Gregario 3',"Wingman's target":'Bersaglio del gregario','Wingman Navlock':'Navlock del gregario',
'Next Subsystem':'Sottosistema successivo','Prev Subsystem':'Sottosistema precedente','Next Jump Dest':'Prossima destinazione di salto','Fire 1':'Fuoco primario','Fire 2':'Fuoco secondario','Next Fire Group':'Gruppo di fuoco successivo','Prev Fire Group':'Gruppo di fuoco precedente','Hardpoints':'Estrai/ritrai armi','Silent Running':'Navigazione silenziosa','Heatsink':'Dissipatore di calore','Lights':'Luci',
'Sensor Range':'Portata sensori','Inc Sensor Range':'Aumenta portata sensori','Dec Sensor Range':'Diminuisci portata sensori','ENG':'Energia ai motori (ENG)','WEP':'Energia alle armi (WEP)','SYS':'Energia ai sistemi (SYS)','RST':'Ripristina distribuzione energia','Reset HMD':'Ricentra visore VR',
'Cargo Scoop':'Portello di carico','Eject All Cargo':'Espelli tutto il carico','Landing Gear':'Carrello di atterraggio','Microphone':'Microfono','SCB':'Cella scudi (SCB)','Chaff':'Chaff','Field Neutraliser':'Neutralizzatore di campo','ECM':'ECM','Weapon Colour':'Colore armi','Engine Colour':'Colore motori','Night Vision':'Visione notturna','Colonisation Module':'Modulo di colonizzazione',
'UI Focus':'Focus interfaccia','Nav Panel':'Pannello navigazione','Comms Panel':'Pannello comunicazioni','Quick Comms':'Comunicazioni rapide','Role Panel':'Pannello ruolo','Systems Panel':'Pannello sistemi','Galaxy Map':'Mappa galattica','GalMap':'Mappa galattica','System Map':'Mappa del sistema','SysMap':'Mappa del sistema','CQC Score':'Punteggio CQC','Toggle Headlook':'Sguardo libero (attiva/disattiva)','Main Menu':'Menu principale','Friends':'Amici','Codex':'Codex','HUD Mode':'Modalità HUD','Enter FSS':'Apri FSS',
'UI Up':'Interfaccia su','UI Down':'Interfaccia giù','UI Left':'Interfaccia sinistra','UI Right':'Interfaccia destra','UI Select':'Interfaccia seleziona','UI Back':'Interfaccia indietro','UI Toggle':'Interfaccia attiva/disattiva','Next Panel':'Pannello successivo','Prev Panel':'Pannello precedente','Next Page':'Pagina successiva','Prev Page':'Pagina precedente',
'Reset Headlook':'Ricentra sguardo','Look Up':'Guarda su','Look Down':'Guarda giù','Look Up/Down':'Guarda su/giù','Look Left':'Guarda a sinistra','Look Right':'Guarda a destra','Look Left/Right':'Guarda sinistra/destra',
'Drive Assist':'Assistenza alla guida','SRV Steer':'Sterzo','Steer Left':'Sterza a sinistra','Steer Right':'Sterza a destra','Vertical Thrusters':'Propulsori verticali','Primary Weapons':'Armi primarie','Secondary Weapons':'Armi secondarie','Handbrake':'Freno a mano','Turret Mode':'Modalità torretta','Turret Yaw':'Torretta sinistra/destra','Turret Left':'Torretta a sinistra','Turret Right':'Torretta a destra','Turret Pitch':'Torretta su/giù','Turret Up':'Torretta su','Turret Down':'Torretta giù',
'Speed':'Velocità','Maximum Speed':'Velocità massima','Zero Speed':'Velocità zero','Inc Speed':'Aumenta velocità','Dec Speed':'Diminuisci velocità','Recall/Dismiss Ship':'Richiama/congeda nave',
'Multicrew Mode':'Modalità multicrew','Primary Utility':'Utilità primaria','Secondary Utility':'Utilità secondaria','Field of View':'Campo visivo','Field of View Out':'Campo visivo più largo','Field of View In':'Campo visivo più stretto','UI Forward':'Interfaccia avanti','UI Backward':'Interfaccia indietro',
'Dock SLF':'Rientro del caccia','Be Defensive':'Atteggiamento difensivo','Be Aggressive':'Atteggiamento aggressivo','Attack My Target':'Attacca il mio bersaglio','Hold Fire':'Cessate il fuoco','Hold Position':'Mantieni posizione','Follow':'Seguimi','Crew Orders':'Ordini all\'equipaggio',
'External Cam':'Telecamera esterna','Prev Cam':'Telecamera precedente','Next Cam':'Telecamera successiva','Free Cam':'Telecamera libera','Toggle HUD':'HUD (attiva/disattiva)','Exit free Cam':'Esci dalla telecamera libera','Advanced Cam':'Telecamera avanzata','Lock to Vehicle':'Aggancia al veicolo','Lock to World':'Aggancia al mondo','Zoom In':'Zoom avanti','Zoom Out':'Zoom indietro','Dec Blur':'Diminuisci sfocatura','Inc Blur':'Aumenta sfocatura',
'Undo Holo-Me':'Annulla (Holo-Me)','Redo Holo-Me':'Ripeti (Holo-Me)','Toggle Holo-Me Rotation':'Rotazione Holo-Me (attiva/disattiva)','Rotate Holo-Me':'Ruota Holo-Me','Play/Pause Audio':'Audio Galnet: riproduci/pausa','Next Audio Track':'Audio Galnet: traccia successiva','Prev Audio Track':'Audio Galnet: traccia precedente','Clear Audio Queue':'Audio Galnet: svuota coda',
'Zoom FSS In':'FSS zoom avanti','Zoom FSS Out':'FSS zoom indietro','Step Zoom FSS In':'FSS zoom avanti a scatti','Step Zoom FSS Out':'FSS zoom indietro a scatti','FSS Tuning':'FSS sintonia','FSS Tune Right':'FSS sintonia a destra','FSS Tune Left':'FSS sintonia a sinistra','FSS Honk':'FSS scansione di scoperta','Exit FSS':'Esci da FSS','Target FSS':'FSS seleziona bersaglio','FSS Help':'FSS aiuto','Toggle Planet Front/Back':'Vista pianeta fronte/retro','Exit DSS':'Esci da DSS',
'Fwd/Back':'Avanti/indietro','Fwd':'Avanti','Back':'Indietro','Strafe':'Passo laterale','Strafe Left':'Passo a sinistra','Strafe Right':'Passo a destra','Turn':'Gira','Turn Left':'Gira a sinistra','Turn Right':'Gira a destra','Sprint':'Corri','Walk':'Cammina','Crouch':'Accovacciati','Jump':'Salta','Interact':'Interagisci','Interact 2':'Interagisci (secondario)','Item Wheel':'Ruota oggetti','Emote Wheel':'Ruota emote','Wheel Mode':'Modalità ruota',
'Item L/R':'Oggetto sinistra/destra','Item Left':'Oggetto a sinistra','Item Right':'Oggetto a destra','Item Up/Dn':'Oggetto su/giù','Item Up':'Oggetto su','Item Down':'Oggetto giù','Fire':'Fuoco','Aim / Zoom':'Mira / zoom','Throw Grenade':'Lancia granata','Melee':'Corpo a corpo','Reload':'Ricarica','Switch WEP':'Cambia arma','WEP 1':'Arma primaria','WEP 2':'Arma secondaria','Select Util':'Arma di utilità','Next WEP':'Arma successiva','Prev WEP':'Arma precedente','Holster':'Riponi arma','Next Grenade':'Granata successiva','Prev Grenade':'Granata precedente','Flashlight':'Torcia','Shields':'Scudi',
'Clear Authority Level':'Azzera livello di autorizzazione','Medkit':'Kit medico','Energy Cell':'Cella di energia','Frag Grenade':'Granata a frammentazione','EMP Grenade':'Granata EMP','Shield Grenade':'Granata a scudo','Recharge Tool':'Strumento di ricarica','Comp Analyser':'Analizzatore di composizione','Suit Tool':'Strumento della tuta','Tool Mode':'Modalità strumento','Mission Help':'Aiuto missione','Ping':'Ping','Open Access Panel':'Apri pannello di accesso','Combat UI':'Interfaccia di combattimento',
'Store Cam Rotation':'Negozio: rotazione telecamera','Store Cam Pitch':'Negozio: telecamera su/giù','Store Cam Yaw':'Negozio: telecamera sinistra/destra','Store Cam Zoom In':'Negozio: zoom avanti','Store Cam Zoom Out':'Negozio: zoom indietro','Store Toggle Preview':'Negozio: anteprima',
'Place Settlement':'Posiziona insediamento','Change Construction Option':'Cambia opzione di costruzione','Rotate Settlement':'Ruota insediamento','Rotate Settlement Left':'Ruota insediamento a sinistra','Rotate Settlement Right':'Ruota insediamento a destra','Exit Placement Cam':'Esci dalla telecamera di posizionamento','Cam Rotation Lock':'Telecamera: blocco rotazione','Cam Reverse':'Telecamera: inverti','Next Genus (DSS)':'Genere successivo (DSS)','Prev Genus (DSS)':'Genere precedente (DSS)'}
CORE={'Yaw':'Imbardata (yaw)','Yaw Left':'Imbardata a sinistra','Yaw Right':'Imbardata a destra','Roll':'Rollio (roll)','Roll Left':'Rollio a sinistra','Roll Right':'Rollio a destra','Pitch':'Beccheggio (pitch)','Pitch Up':'Beccheggio su','Pitch Down':'Beccheggio giù',
'Lateral Thrust':'Spinta laterale','Thrust Left':'Spinta a sinistra','Thrust Right':'Spinta a destra','Vertical Thrust':'Spinta verticale','Thrust Up':'Spinta su','Thrust Down':'Spinta giù','Forward Thrust':'Spinta avanti','Forward Thrust (axis)':'Spinta avanti/indietro (asse)','Reverse thrust':'Spinta indietro','Reverse Thrust':'Spinta indietro',
'X':'asse X','Y':'asse Y','Z':'asse Z','Z Hold':'asse Z (tieni premuto)','Up':'su','Down':'giù','Left':'a sinistra','Right':'a destra','Forward':'avanti','Forwards':'avanti','Backward':'indietro','Backwards':'indietro','Zoom':'zoom','Zoom In':'zoom avanti','Zoom Out':'zoom indietro','Home':'posizione iniziale','Up (axis)':'su (asse)','Down (axis)':'giù (asse)'}
def tr(en):
    if en in FULL: return FULL[en]
    m=re.match(r'^(\d+)% (Reverse|Throttle)$',en)
    if m: return ('Manetta ' if m.group(2)=='Throttle' else 'Retromarcia ')+m.group(1)+'%'
    m=re.match(r'^Emote (\d)$',en)
    if m: return 'Emote '+m.group(1)
    m=re.match(r'^Cam - (.*)$',en)
    if m: return 'Telecamera: '+{'Cockpit Front':'cabina davanti','Cockpit Back':'cabina dietro','Commander 1':'comandante 1','Commander 2':'comandante 2','Co-Pilot 1':'copilota 1','Co-Pilot 2':'copilota 2','Front':'davanti','Back':'dietro','Low':'dal basso','Vehicle Turret':'torretta del veicolo'}.get(m.group(1),m.group(1))
    suf=''
    for a,b in ((' (Lndg)',' (atterraggio)'),(' (Alt)',' (alternativo)'),(' (DSS)',' (DSS)')):
        if en.endswith(a): en=en[:-len(a)]; suf=b
    en=en.strip()
    for pre,it in (('GalMap ','Mappa galattica: '),('Placement Cam ','Telecamera di posizionamento: '),('Cam ','Telecamera: '),('FSS ','FSS: '),('DSS ','DSS: '),('Inc Placement Cam ','Aumenta: telecamera di posizionamento '),('Dec Placement Cam ','Diminuisci: telecamera di posizionamento '),('Inc Cam ','Aumenta: telecamera '),('Dec Cam ','Diminuisci: telecamera ')):
        if en.startswith(pre):
            rest=en[len(pre):].strip(); core=FULL.get(rest) or CORE.get(rest) or {'Speed':'velocità'}.get(rest)
            if core: return it+core[0].lower()+core[1:]+suf
    if en in FULL: return FULL[en]+suf
    if en in CORE: return CORE[en]+suf
    return None
def group(tag,g):
    if tag.endswith('_Landing'): return 'Nave - Atterraggio'
    if 'Placement' in tag or 'Settlement' in tag or tag=='ChangeConstructionOption': return 'Colonizzazione'
    if tag.startswith('Humanoid') or tag.endswith('_Humanoid'): return 'A piedi'
    if 'Buggy' in tag or g=='SRV': return 'SRV (veicolo di superficie)'
    if g is None:
        if 'Cam' in tag: return 'Telecamera'
        if 'SAA' in tag or 'FSS' in tag: return 'Scanner (FSS / DSS)'
    return {'Ship':'Nave','Misc':'Varie','UI':'Interfaccia','Head look':'Sguardo libero','Galaxy map':'Mappa galattica','Multicrew':'Multicrew','Fighter':'Caccia imbarcato','Camera':'Telecamera','Holo-Me':'Holo-Me','Scanners':'Scanner (FSS / DSS)','OnFoot':'A piedi','SRV':'SRV (veicolo di superficie)'}.get(g,'Nave' if g is None else g)
out=[]; miss=[]
for e in root:
    if not len(e): continue
    t=e.tag; kids=[c.tag for c in e]
    kind='axis' if 'Binding' in kids else ('toggle' if 'ToggleOn' in kids else 'button')
    en=FIX.get(t) or (ref.get(t) or {}).get('Name','').strip() or EXTRA.get(t) or t
    it=tr(en)
    if not it: miss.append((t,en)); it=en
    out.append({'tag':t,'en':en,'it':it,'group':group(t,(ref.get(t) or {}).get('Group')),'kind':kind})
json.dump(out,open(sys.argv[3],'w'),ensure_ascii=False,separators=(',',':'))
print(len(out),'senza traduzione:',miss)
import collections; print(collections.Counter(o['group'] for o in out)); print(collections.Counter(o['kind'] for o in out))
for o in out[::9]: print(o['tag'],'|',o['en'],'|',o['it'],'|',o['group'])
