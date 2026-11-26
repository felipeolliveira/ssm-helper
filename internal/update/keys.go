package update

// publicKeys verify the signature of SHA256SUMS.txt (base64, ed25519). The
// first is the primary key, whose private half is the UPDATE_SIGNING_KEY
// secret of the release workflow. The second is the backup key, whose private
// half is kept only outside GitHub: if the primary is lost or leaked, releases
// signed with the backup are still accepted, and a new version replaces the
// primary. See "Signing releases" in the README.
//
// Generate them with: go run ./tools/updatekey generate -out <name>
var publicKeys = []string{
	"8E7uxXxehe6a0rnys1eUQUlRnK8zH7unHSNbcpO3nhA=", // primary
	"2+9RXXD2K42XPnvZBi2ecPs9y9rInrrU37Y9O3+JJ+c=", // backup
}
