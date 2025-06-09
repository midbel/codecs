<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="var">
			<lang>go</lang>
		</xsl:variable>
		<language>
			<xsl:value-of select="$var"/>
		</language>
	</xsl:template>
</xsl:stylesheet>