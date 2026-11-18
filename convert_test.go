package greeklish

import (
	"os"
	"sync"
	"testing"
)

var testModel = sync.OnceValues(func() (*Model, error) {
	data, err := os.ReadFile("model/greeklish.bin")
	if err != nil {
		return nil, err
	}
	m := &Model{}
	return m, m.UnmarshalBinary(data)
})

func loadModel(t testing.TB) *Model {
	t.Helper()
	m, err := testModel()
	if err != nil {
		t.Fatalf("model missing, run make model: %v", err)
	}
	return m
}

type testCase struct{ in, want string }

func runCases(t *testing.T, cases []testCase) {
	m := loadModel(t)
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := m.Convert(c.in); got != c.want {
				t.Errorf("\n in:   %q\n got:  %q\n want: %q", c.in, got, c.want)
			}
		})
	}
}

func TestUserExamples(t *testing.T) {
	runCases(t, []testCase{
		{"gia sou ime o alexandros.", "Γεια σου είμαι ο Αλέξανδρος."},
		{"kosta, I ekdosi ine online 1.4.2 na to kanw release sto staging ?", "Κώστα, η έκδοση είναι online 1.4.2 να το κάνω release στο staging;"},
	})
}

func TestEverydayChat(t *testing.T) {
	runCases(t, []testCase{
		{"kalimera ti kaneis?", "Καλημέρα τι κάνεις;"},
		{"kala eimai esy?", "Καλά είμαι εσύ;"},
		{"pou eisai re?", "Πού είσαι ρε;"},
		{"tha argisw ligo", "Θα αργήσω λίγο"},
		{"den kserw ti na pw", "Δεν ξέρω τι να πω"},
		{"efxaristw poli", "Ευχαριστώ πολύ"},
		{"kalo taxidi", "Καλό ταξίδι"},
		{"pame gia kafe meta ti douleia?", "Πάμε για καφέ μετά τη δουλειά;"},
		{"exeis dikio", "Έχεις δίκιο"},
		{"mporeis na me pareis tilefono?", "Μπορείς να με πάρεις τηλέφωνο;"},
		{"to vradi tha eimaste spiti", "Το βράδυ θα είμαστε σπίτι"},
		{"ti ora einai?", "Τι ώρα είναι;"},
		{"xronia polla kai na eisai panta kala", "Χρόνια πολλά και να είσαι πάντα καλά"},
		{"kali xronia se olous", "Καλή χρονιά σε όλους"},
		{"pws se lene?", "Πώς σε λένε;"},
		{"i mama mou einai giatros", "Η μαμά μου είναι γιατρός"},
		{"o kairos simera einai poli kalos", "Ο καιρός σήμερα είναι πολύ καλός"},
		{"ela edw", "Έλα εδώ"},
		{"nistazw poli", "Νυστάζω πολύ"},
		{"pinaw san likos", "Πεινάω σαν λύκος"},
		{"tha se parw tilefono to apogeuma", "Θα σε πάρω τηλέφωνο το απόγευμα"},
		{"mou leipeis poli", "Μου λείπεις πολύ"},
		{"ola kala edw, esu pws eisai?", "Όλα καλά εδώ, εσύ πώς είσαι;"},
		{"sygnwmi gia tin kathisterisi", "Συγνώμη για την καθυστέρηση"},
		{"den pirazei", "Δεν πειράζει"},
		{"ti tha fame simera?", "Τι θα φάμε σήμερα;"},
		{"eimai sto dromo, ftanw se deka lepta", "Είμαι στο δρόμο, φτάνω σε δέκα λεπτά"},
		{"kalinixta kai oneira glyka", "Καληνύχτα και όνειρα γλυκά"},
		{"to paidi koimatai", "Το παιδί κοιμάται"},
		{"prepei na fugw twra", "Πρέπει να φύγω τώρα"},
	})
}

func TestSpellingStyles(t *testing.T) {
	runCases(t, []testCase{
		{"thelw na faw", "Θέλω να φάω"},
		{"8elw na faw", "Θέλω να φάω"},
		{"den 3erw", "Δεν ξέρω"},
		{"den xerw", "Δεν ξέρω"},
		{"kalhmera", "Καλημέρα"},
		{"kalimera", "Καλημέρα"},
		{"eyxaristw", "Ευχαριστώ"},
		{"euxaristo", "Ευχαριστώ"},
		{"ine kalo", "Είναι καλό"},
		{"eine kalo", "Είναι καλό"},
		{"einai kalo", "Είναι καλό"},
		{"pou pas?", "Πού πας;"},
		{"pu pas?", "Πού πας;"},
		{"ti ginete?", "Τι γίνεται;"},
		{"ti ginetai?", "Τι γίνεται;"},
		{"oi filoi mou", "Οι φίλοι μου"},
		{"mpravo sou", "Μπράβο σου"},
		{"bravo sou", "Μπράβο σου"},
		{"ntropi", "Ντροπή"},
		{"dropi", "Ντροπή"},
		{"ayrio", "Αύριο"},
		{"avrio", "Αύριο"},
		{"psomi", "Ψωμί"},
		{"xarika poly", "Χάρηκα πολύ"},
		{"harika poli", "Χάρηκα πολύ"},
		{"exw douleia", "Έχω δουλειά"},
		{"exo duleia", "Έχω δουλειά"},
		{"ksekinaw twra", "Ξεκινάω τώρα"},
		{"yia sou", "Γεια σου"},
		{"geia sou", "Γεια σου"},
	})
}

func TestTypos(t *testing.T) {
	runCases(t, []testCase{
		{"kalimeea", "Καλημέρα"},
		{"efxarito poli", "Ευχαριστώ πολύ"},
		{"dne kserw", "Δεν ξέρω"},
		{"kalispeera", "Καλησπέρα"},
		{"thelw na paw spti", "Θέλω να πάω σπίτι"},
		{"einai poli wraio", "Είναι πολύ ωραίο"},
		{"einia poli kalo", "Είναι πολύ καλό"},
		{"sinantisi stis deka", "Συνάντηση στις δέκα"},
		{"to aftokinito xalase", "Το αυτοκίνητο χάλασε"},
		{"alos enas", "Άλλος ένας"},
		{"pragmatika", "Πραγματικά"},
	})
}

func TestMixedEnglishAndCode(t *testing.T) {
	runCases(t, []testCase{
		{"to build espase sto CI", "Το build έσπασε στο CI"},
		{"kane merge to PR otan mporeis", "Κάνε merge το PR όταν μπορείς"},
		{"esteila email ston giorgo", "Έστειλα email στον Γιώργο"},
		{"o server einai down apo to prwi", "Ο server είναι down από το πρωί"},
		{"des to https://example.com/docs kai pes mou", "Δες το https://example.com/docs και πες μου"},
		{"to arxeio config.yaml leipei", "Το αρχείο config.yaml λείπει"},
		{"steile mou sto alex@example.com", "Στείλε μου στο alex@example.com"},
		{"to deploy egine stis 12:30", "Το deploy έγινε στις 12:30"},
		{"trexe to npm install prwta", "Τρέξε το npm install πρώτα"},
		{"anoikse ena ticket sto github", "Άνοιξε ένα ticket στο github"},
		{"exw meeting meta", "Έχω meeting μετά"},
		{"to laptop mou einai poli argo", "Το laptop μου είναι πολύ αργό"},
		{"to fix einai sto branch feature/login", "Το fix είναι στο branch feature/login"},
		{"this is english and stays english", "this is english and stays english"},
		{"ok", "ok"},
		{"OK", "OK"},
		{"ok tha to dw", "ok θα το δω"},
	})
}

func TestGreekInput(t *testing.T) {
	runCases(t, []testCase{
		{"καλημερα τι κανεις", "Καλημέρα τι κάνεις"},
		{"ειμαι καλα", "Είμαι καλά"},
		{"θελω να παω σπιτι", "Θέλω να πάω σπίτι"},
		{"δεν ξερω", "Δεν ξέρω"},
		{"η εκδοση ειναι ετοιμη", "Η έκδοση είναι έτοιμη"},
		{"ειμε καλα", "Είμαι καλά"},
		{"καλιμερα", "Καλημέρα"},
		{"ευχαριστω πολυ", "Ευχαριστώ πολύ"},
		{"Γεια σου, τι κάνεις;", "Γεια σου, τι κάνεις;"},
		{"Η Μαρία πήγε στην Αθήνα.", "Η Μαρία πήγε στην Αθήνα."},
		{"Πότε θα έρθεις;", "Πότε θα έρθεις;"},
		{"το σπιτι μου ειναι μακρυα", "Το σπίτι μου είναι μακριά"},
	})
}

func TestCasingAndNames(t *testing.T) {
	runCases(t, []testCase{
		{"GEIA SOU", "ΓΕΙΑ ΣΟΥ"},
		{"pame athina me ti maria", "Πάμε Αθήνα με τη Μαρία"},
		{"menw sti thessaloniki", "Μένω στη Θεσσαλονίκη"},
		{"o nikos kai i eleni", "Ο Νίκος και η Ελένη"},
		{"sti kriti kanei zesti", "Στην Κρήτη κάνει ζέστη"},
	})
}

func TestPunctuationAndLayout(t *testing.T) {
	runCases(t, []testCase{
		{"ti kaneis ?", "Τι κάνεις;"},
		{"ela,pame", "Έλα, πάμε"},
		{"kalimera. ti kaneis? ola kala", "Καλημέρα. Τι κάνεις; Όλα καλά"},
		{"kalimera\nti kaneis", "Καλημέρα\nΤι κάνεις"},
		{"(ela)", "(Έλα)"},
		{"«ela»", "«Έλα»"},
		{"perimene...", "Περίμενε..."},
		{"wraia!!!", "Ωραία!!!"},
		{"s'agapw", "Σ' αγαπώ"},
	})
}

func TestEdgeCases(t *testing.T) {
	runCases(t, []testCase{
		{"", ""},
		{"   ", ""},
		{"1.4.2", "1.4.2"},
		{"kalimera 😀", "Καλημέρα 😀"},
		{"?", ";"},
		{"mp3 h264 v1.2", "mp3 h264 v1.2"},
		{"#hashtag @mention", "#hashtag @mention"},
	})
}

func TestLongMessages(t *testing.T) {
	runCases(t, []testCase{
		{"kalimera paidia, simera tha argisw ligo giati exei poli kinisi stin ethniki odo",
			"Καλημέρα παιδιά, σήμερα θα αργήσω λίγο γιατί έχει πολύ κίνηση στην εθνική οδό"},
		{"mporeis na mou steileis to arxeio otan exeis xrono? to xreiazomai mexri to vradi",
			"Μπορείς να μου στείλεις το αρχείο όταν έχεις χρόνο; Το χρειάζομαι μέχρι το βράδυ"},
		{"exw ena provlima me ton ypologisti, den anoigei kai den kserw ti na kanw",
			"Έχω ένα πρόβλημα με τον υπολογιστή, δεν ανοίγει και δεν ξέρω τι να κάνω"},
		{"o pateras mou leei oti prepei na diavazw perissotero",
			"Ο πατέρας μου λέει ότι πρέπει να διαβάζω περισσότερο"},
		{"to savvatokyriako tha pame sto xwrio na doume th giagia",
			"Το σαββατοκύριακο θα πάμε στο χωριό να δούμε τη γιαγιά"},
		{"ta leme thn deutera", "Τα λέμε την Δευτέρα"},
		{"an thes ela to apogeuma apo to grafeio na ta poume",
			"Αν θες έλα το απόγευμα από το γραφείο να τα πούμε"},
	})
}

func TestColloquial(t *testing.T) {
	runCases(t, []testCase{
		{"re si ti les", "Ρε συ τι λες"},
		{"ela re file", "Έλα ρε φίλε"},
		{"pame mia volta?", "Πάμε μια βόλτα;"},
		{"kala na pathei", "Καλά να πάθει"},
		{"den variesai", "Δεν βαριέσαι"},
		{"ti na sou pw", "Τι να σου πω"},
		{"ola kala?", "Όλα καλά;"},
		{"min aghoneisai", "Μην αγχώνεσαι"},
		{"8a sou pw meta", "Θα σου πω μετά"},
		{"isws na erthw ki egw", "Ίσως να έρθω κι εγώ"},
	})
}

func TestHarderTypos(t *testing.T) {
	runCases(t, []testCase{
		{"efxaristw pou me voithises", "Ευχαριστώ που με βοήθησες"},
		{"dne exw idea", "Δεν έχω ιδέα"},
		{"kalimerra se olous", "Καλημέρα σε όλους"},
		{"tha se dw aurio", "Θα σε δω αύριο"},
		{"poso kanei to eisitirio?", "Πόσο κάνει το εισιτήριο;"},
		{"exeis tilefono tou giwrgou?", "Έχεις τηλέφωνο του Γιώργου;"},
		{"sigoura?", "Σίγουρα;"},
		{"gamatos", "Γαμάτος"},
	})
}

func TestGreekGrammarSlips(t *testing.T) {
	runCases(t, []testCase{
		{"πηγα στη Κρήτη", "Πήγα στην Κρήτη"},
		{"δε ξέρω", "Δεν ξέρω"},
		{"δε έχω χρόνο", "Δεν έχω χρόνο"},
		{"βλέπω τη ταινία", "Βλέπω την ταινία"},
		{"θελω να το κανω σημερα", "Θέλω να το κάνω σήμερα"},
		{"τα παιδια παιζουν εξω", "Τα παιδιά παίζουν έξω"},
		{"πολυ ωραιο", "Πολύ ωραίο"},
		{"που εισαι;", "Πού είσαι;"},
		{"πως σε λενε;", "Πώς σε λένε;"},
		{"ο δρομος που μενω", "Ο δρόμος που μένω"},
	})
}

func TestCodeSwitchingHeavy(t *testing.T) {
	runCases(t, []testCase{
		{"to PR exei conflicts, kane rebase sto main", "Το PR έχει conflicts, κάνε rebase στο main"},
		{"to pipeline pernaei local alla oxi sto CI", "Το pipeline περνάει local αλλά όχι στο CI"},
		{"kleise to issue #42 otan ginei deploy", "Κλείσε το issue #42 όταν γίνει deploy"},
		{"to docker container den kanei start", "Το docker container δεν κάνει start"},
		{"exw call me ton client stis 5", "Έχω call με τον client στις 5"},
		{"steile mou to link sto slack", "Στείλε μου το link στο slack"},
		{"to API epistrefei 500 se kathe request", "Το API επιστρέφει 500 σε κάθε request"},
		{"kane ena git pull prwta", "Κάνε ένα git pull πρώτα"},
	})
}
