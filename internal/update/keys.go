package update

// publicKeys verify the signature of SHA256SUMS.txt (base64, ed25519). The
// first is the primary key, whose private half is the UPDATE_SIGNING_KEY
// secret of the release workflow. The second is the backup key, whose private
// half is kept only outside GitHub: if the primary is lost or leaked, releases
// signed with the backup are still accepted, and a new version replaces the
// primary. See "Signing releases" in DEVELOPMENT.md.
//
// Generate them with: go run ./tools/updatekey generate -out <name>
var publicKeys = []string{
	"K/4kQ6S5OhAmax81hAUcZZQ3ALLiWLdjXXf2a4z1Gb8=", // primary
	"dpXRa+8rDePot7ltyHwAu7qhuJdFWMuatGsAmcKhAmk=", // backup
}
