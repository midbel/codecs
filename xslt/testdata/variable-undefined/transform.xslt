<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="var" select="/root/language[1]"/>
		<lang>
			<xsl:value-of select="$test"/>
		</lang>
	</xsl:template>
</xsl:stylesheet>